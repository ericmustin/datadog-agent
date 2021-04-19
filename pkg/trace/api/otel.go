// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package api

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/datadog-agent/pkg/trace/traceutil"
)

const (
	attributeContainerID              string = "container.id"
	attributeDeploymentEnvironment    string = "deployment.environment"
	attributeDatadogEnvironment       string = "env"
	attributeExceptionEventName       string = "exception"
	attributeExceptionMessage         string = "exception.message"
	attributeExceptionStacktrace      string = "exception.stacktrace"
	attributeExceptionType            string = "exception.type"
	attributeHTTPMethod               string = "http.method"
	attributeHTTPRoute                string = "http.route"
	attributeHTTPStatusCode           string = "http.status_code"
	attributeK8sPod                   string = "k8s.pod.name"
	attributeMessagingOperation       string = "messaging.operation"
	attributeMessagingDestination     string = "messaging.destination"
	attributeRPCMethod                string = "rpc.method"
	attributeRPCService               string = "rpc.service"
	attributeServiceName              string = "service.name"
	attributeServiceVersion           string = "service.version"
	attributeSpanAnalyticsEvent       string = "analytics.event"
	attributeSpanEventSampleRate      string = "_dd1.sr.eausr"
	attributeSpanSamplingPriority     string = "sampling.priority"
	attributeSpanType                 string = "span.type"
	instrumentationLibraryName        string = "otel.library.name"
	instrumentationLibraryVersion     string = "otel.library.version"
	keySamplingPriority               string = "_sampling_priority_v1"
	versionTag                        string = "version"
	oldILNameTag                      string = "otel.instrumentation_library.name"
	currentILNameTag                  string = "otel.library.name"
	errorCode                         int32  = 1
	okCode                            int32  = 0
	httpKind                          string = "http"
	webKind                           string = "web"
	customKind                        string = "custom"
	grpcPath                          string = "grpc.path"
	eventsTag                         string = "events"
	eventNameTag                      string = "name"
	eventAttrTag                      string = "attributes"
	eventTimeTag                      string = "time"
	otelOkCode                        int32 = 1
	otelErrorCode                     int32 = 2
	resourceNoServiceName             string = "OTLPResourceNoServiceName"
	tagErrorType                      string = "error.type"
	tagErrorMsg                       string = "error.msg"
	tagErrorStack                     string = "error.stack"
	tagW3CTraceState                  string = "w3c.tracestate"
	// tagContainersTags specifies the name of the tag which holds key/value
	// pairs representing information about the container (Docker, EC2, etc).
	tagContainersTags = "_dd.tags.container"

)

var spanKindMap = map[int32]string{
	0: "SPAN_KIND_UNSPECIFIED",
	1: "SPAN_KIND_INTERNAL",
	2: "SPAN_KIND_SERVER",
	3: "SPAN_KIND_CLIENT",
	4: "SPAN_KIND_PRODUCER",
	5: "SPAN_KIND_CONSUMER",
}

// converts a Trace's resource spans into a trace payload
// gets a service name tag and an tags hash seeded with resource attributes
// gets environment name tag from tags 
// TODO: gets config as falllback
// loop over instrumentationn library span batch:
//   - extract instrumentationn library name and version from json and apply to tags
//   -  loop over span batch:
// 		- convert span to datadog span, passing in tags and config
//			- fill in details and translation but broadly:
// 				- trace_id, span_id, parent_span_id, start / duration in nano, meta, metrics, status, tags, err tags, resource, op name, kind/type , unified service tags
// 		- add to trace map 
// export trace map as pb.Traces []*Trace
func OtelResourceSpansToDatadogSpans(rs map[string]interface{}) []*pb.Span {
	// get env tag
	// env := cfg.Env
	traces := []*pb.Span{}

	resourceraw := rs["resource"]
	ilsraw := rs["instrumentationLibrarySpans"]

	// if resource.Attributes().Len() == 0 && ilss.Len() == 0 {
	// 	return payload
	// }

	if spans, ilok := ilsraw.([]interface{}); ilok {
		if resource, rok := resourceraw.(map[string]interface{}); rok {
			resourceServiceName, datadogTags := resourceToDatadogServiceNameAndAttributeMap(resource)
			extractDatadogEnv(datadogTags)

			if attributes, aok := resource["attributes"].([]interface{}); aok {
				if len(attributes) == 0 && len(spans) == 0 {
					log.Errorf("no traces in otey payload")
				} else {

					if len(spans) > 0 {
						for i := 0; i < len(spans); i++ { 
							ilspan := spans[i].(map[string]interface{})
							// get IL info

							// extractInstrumentationLibraryTags(ilspan.InstrumentationLibrary(), datadogTags)
							il := ilspan["instrumentationLibrary"].(map[string]interface{})
							extractInstrumentationLibraryTags(il, datadogTags)

							ilSpanList := ilspan["spans"].([]interface{})

							for j := 0; j < len(ilSpanList); j++ { 
								span := ilSpanList[j].(map[string]interface{})
								ddSpan := spanToDatadogSpan(span, resourceServiceName, datadogTags)
								traces = append(traces, ddSpan)
							}
						}
					}
				}
			}
		}
	}

	log.Errorf("returning %s traces", len(traces))
	return traces
}


func resourceToDatadogServiceNameAndAttributeMap(
	resource map[string]interface{},
) (serviceName string, datadogTags map[string]string) {
	// attrs := resource.Attributes()
	// log.Errorf("ok and %s", resource["attributes"])
	if attributes, aok := resource["attributes"].([]interface{}); aok {
		attrs := attributes
		
		// log.Errorf("atttrs %s", attributes)
		// predefine capacity where possible with extra for _dd.tags.container payload		
		datadogTags = make(map[string]string, len(attrs) + 1)

		if len(attrs) == 0 {
			return resourceNoServiceName, datadogTags
		}

		for i := 0; i < len(attrs); i++ { 
			v := attrs[i].(map[string]interface{})
			key := v["key"].(string)

			if valueMap, vok := v["value"].(map[string]interface{}); vok {
				value := AttributeValueToString(valueMap)
				datadogTags[key] = value				
			}
		}
		
		serviceName = extractDatadogServiceName(datadogTags)

		return serviceName, datadogTags
	} else {
		datadogTags = make(map[string]string, 1)
		serviceName = extractDatadogServiceName(datadogTags)

		return serviceName, datadogTags
	}	

}

func extractDatadogServiceName(datadogTags map[string]string) string {
	var serviceName string
	if sn, ok := datadogTags[attributeServiceName]; ok {
		serviceName = sn
		delete(datadogTags, attributeServiceName)
	} else {
		serviceName = resourceNoServiceName
	}
	return serviceName
}

func extractDatadogEnv(datadogTags map[string]string) {
	// specification states that the resource level deployment.environment should be used for passing env, so defer to that
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/deployment_environment.md#deployment
	if resourceEnv, ok := datadogTags[attributeDeploymentEnvironment]; ok {
		// if .env tag is already set we don't override as that is datadog default
		if _, ok := datadogTags[attributeDatadogEnvironment]; !ok {
			datadogTags[attributeDatadogEnvironment] = resourceEnv
		}
	}
}

func extractInstrumentationLibraryTags(il map[string]interface{}, datadogTags map[string]string) {
	if ilName := il["name"]; ilName != "" {
		datadogTags[instrumentationLibraryName] = ilName.(string)
	}
	if ilVer := il["version"]; ilVer != "" {
		datadogTags[instrumentationLibraryVersion] = ilVer.(string)
	}
}

// convertSpan takes an internal span representation and returns a Datadog span.
func spanToDatadogSpan(s map[string]interface{},
	serviceName string,
	datadogTags map[string]string,
) *pb.Span {
	tags := aggregateSpanTags(s, datadogTags)

	// TODO: handle config for service name
	// // otel specification resource service.name takes precedence
	// // and configuration DD_ENV as fallback if it exists
	// if cfg.Service != "" {
	// 	// prefer the collector level service name over an empty string or otel default
	// 	if serviceName == "" || serviceName == tracetranslator.ResourceNoServiceName {
	// 		serviceName = cfg.Service
	// 	}
	// }

	// TODO: determine lang via headers? or in otel is that a corresponding value in resource?
	normalizedServiceName, _ := traceutil.NormalizeService(serviceName, "")

	//  canonical resource attribute version should override others if it exists
	if rsTagVersion := tags[attributeServiceVersion]; rsTagVersion != "" {
		tags[versionTag] = rsTagVersion
	} else {

		// TODO: handle config for version
		// // if no version tag exists, set it if provided via config
		// if cfg.Version != "" {
		// 	if tagVersion := tags[versionTag]; tagVersion == "" {
		// 		tags[versionTag] = cfg.Version
		// 	}
		// }
	}

	// get tracestate as just a general tag
	// TODO: handle trace state
	// if len(s.TraceState()) > 0 {
	// 	tags[tagW3CTraceState] = string(s.TraceState())
	// }

	// TODO: handle events and error tag in events
	// // get events as just a general tag
	// if s.Events().Len() > 0 {
	// 	tags[eventsTag] = eventsToString(s.Events())
	// }
	var startTime int64
	var endTime int64
	var duration int64

	if startTimeMap, stmok := s["startTimeUnixNano"]; stmok {
		startMid := startTimeMap.(float64)
		startTime = int64(startMid)
	} else {
		log.Errorf("start time error %s", startTimeMap)
	}

	if endTimeMap, stmok := s["endTimeUnixNano"]; stmok {
		endTime = int64(endTimeMap.(float64))
	} else {
		log.Errorf("end time error %s", endTimeMap)
	}

	if endTime == 0 {
		log.Errorf("error endtime, start %s %s", endTime, startTime)
		duration = 0
	} else {
		duration = endTime - startTime	
	}

	// // by checking for error and setting error tags before creating datadog span
	// // we can then set Error field when creating and predefine a max meta capacity
	status := s["status"].(map[string]interface{})

	log.Errorf("status is type %T and looks like %s", status, status)
	isSpanError := getSpanErrorAndSetTags(s, tags)

	var spanId string
	var traceId string
	var parentSpanId string	

	if traceIdMap, tiok := s["traceId"]; tiok {
		traceId = traceIdMap.(string)
	} else {
		log.Errorf("error traceId, %s", s["traceId"])
	}

	if spanIdMap, tiok := s["spanId"]; tiok {
		spanId = spanIdMap.(string)
	} else {
		log.Errorf("error spanId, %s", s["spanId"])
	}

	formattedTraceId := uint64(binary.BigEndian.Uint64([]byte(traceId[:])))
	formattedSpanId := uint64(binary.BigEndian.Uint64([]byte(spanId[:])))

	span := &pb.Span{
		TraceID:  formattedTraceId,
		SpanID:   formattedSpanId,
		Name:     getDatadogSpanName(s, tags),
		Resource: getDatadogResourceName(s, tags),		
		Service:  normalizedServiceName,
		Start:    int64(startTime),
		Duration: duration,
		Metrics:  map[string]float64{},
		Meta:     make(map[string]string, len(tags)),
		Type:     spanKindToDatadogType(NormalizeSpanKind(int32(s["kind"].(float64)))),
		Error:    isSpanError,
	}

	if parentSpanIdMap, psiok := s["parentSpanId"]; psiok {
		parentSpanId = parentSpanIdMap.(string)
		span.ParentID = uint64(binary.BigEndian.Uint64([]byte(parentSpanId[:])))
	}

	// // Set Attributes as Tags
	for key, val := range tags {
		setStringTag(span, key, val)
	}

	return span
}

func aggregateSpanTags(span map[string]interface{}, datadogTags map[string]string) map[string]string {
	// predefine capacity as at most the size attributes and global tags
	// there may be overlap between the two.

	spanAttributes := span["attributes"].([]interface{})
	spanTags := make(map[string]string, len(spanAttributes) + len(datadogTags))

	for key, val := range datadogTags {
		spanTags[key] = val
	}

	for i := 0; i < len(spanAttributes); i++ { 
		attributeMap := spanAttributes[i].(map[string]interface{})
		key := attributeMap["key"].(string)

		if valueMap, vok := attributeMap["value"].(map[string]interface{}); vok {
			value := AttributeValueToString(valueMap)
			spanTags[key] = value
		}
	}

	// TODO: does this get done later in the pipeline?
	spanTags[tagContainersTags] = buildDatadogContainerTags(spanTags)
	return spanTags
}

// buildDatadogContainerTags returns container and orchestrator tags belonging to containerID
// as a comma delimeted list for datadog's special container tag key
func buildDatadogContainerTags(spanTags map[string]string) string {
	var b strings.Builder

	if val, ok := spanTags[attributeContainerID]; ok {
		b.WriteString(fmt.Sprintf("%s:%s,", "container_id", val))
	}
	if val, ok := spanTags[attributeK8sPod]; ok {
		b.WriteString(fmt.Sprintf("%s:%s,", "pod_name", val))
	}

	return strings.TrimSuffix(b.String(), ",")
}

// TODO: this is a hack, how can we import or vendor otel helpers
// AttributeValueToString converts an OTLP AttributeValue object to its equivalent string representation
func AttributeValueToString(attr map[string]interface{}) string {
	var modifiedvalue string
	for key, value := range attr {
		switch key {
		case "stringValue":
			modifiedvalue = value.(string)
		case "intValue":
			modifiedvalue =  strconv.Itoa(value.(int))
		case "doubleValue":
			modifiedvalue = strconv.FormatFloat(value.(float64), 'f', 6, 64)
		case "boolValue":
			modifiedvalue = strconv.FormatBool(value.(bool))
		case "arrayValue":
			jsonStr, _ := json.Marshal(value)
			modifiedvalue = string(jsonStr)
		case "mapValue":
			jsonStr, _ := json.Marshal(value)
			modifiedvalue = string(jsonStr)
		default:
		   modifiedvalue = "unknown"
		}
	}

	return modifiedvalue
}


func decodeAPMId(id string) uint64 {
	if len(id) > 16 {
		id = id[len(id)-16:]
	}

	val, err := strconv.ParseUint(id, 16, 64)

	if err != nil {
		return 0
	}

	return val
}

func setMetric(s *pb.Span, key string, v float64) {
	switch key {
	case attributeSpanSamplingPriority:
		s.Metrics[keySamplingPriority] = v
	default:
		s.Metrics[key] = v
	}
}

func setStringTag(s *pb.Span, key, v string) {
	switch key {
	// if a span has `service.name` set as the tag
	case attributeServiceName:
		s.Service = v
	case attributeSpanType:
		s.Type = v
	case attributeSpanAnalyticsEvent:
		if v != "false" {
			setMetric(s, attributeSpanEventSampleRate, 1)
		} else {
			setMetric(s, attributeSpanEventSampleRate, 0)
		}
	case "http.status_code":
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Errorf("cant set status code %s %s", v, err)
			s.Meta[key] = v
		} else {
			setMetric(s, key, f)	
		}
	default:
		s.Meta[key] = v
	}
}

func getDatadogSpanName(span map[string]interface{}, datadogTags map[string]string) string {
	// largely a port of logic here
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L213
	// Get span name by using instrumentation library name and span kind while backing off to span.kind

	// The spec has changed over time and, depending on the original exporter, IL Name could represented a few different ways
	// so we try to account for all permutations
	if ilnOtlp, okOtlp := datadogTags[instrumentationLibraryName]; okOtlp {
		name, _ := traceutil.NormalizeName(fmt.Sprintf("%s.%s", ilnOtlp, NormalizeSpanKind(int32(span["kind"].(float64)))))
		
		return traceutil.NormalizeTag(name)
	}

	if ilnOtelCur, okOtelCur := datadogTags[currentILNameTag]; okOtelCur {
		name, _ := traceutil.NormalizeName(fmt.Sprintf("%s.%s", ilnOtelCur, NormalizeSpanKind(int32(span["kind"].(float64)))))
		
		return traceutil.NormalizeTag(name)
	}

	if ilnOtelOld, okOtelOld := datadogTags[oldILNameTag]; okOtelOld {
		name, _ := traceutil.NormalizeName(fmt.Sprintf("%s.%s", ilnOtelOld, NormalizeSpanKind(int32(span["kind"].(float64)))))
		
		return traceutil.NormalizeTag(name)
	}

	name, _ := traceutil.NormalizeName(fmt.Sprintf("%s.%s", "opentelemetry", NormalizeSpanKind(int32(span["kind"].(float64)))))
	
	return traceutil.NormalizeTag(name)
}

// NormalizeSpanKind returns a span kind with the SPAN_KIND prefix trimmed off
func NormalizeSpanKind(kind int32) string {
	if kindString, ok := spanKindMap[kind]; ok {
		return strings.TrimPrefix(kindString, "SPAN_KIND_")
	}
	
	return "UNKNOWN"
}

func getDatadogResourceName(s map[string]interface{}, datadogTags map[string]string) string {
	// largely a port of logic here
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L229
	// Get span resource name by checking for existence http.method + http.route 'GET /api'
	// Also check grpc path as fallback for http requests
	// backing off to just http.method, and then span.name if unrelated to http
	if method, methodOk := datadogTags[attributeHTTPMethod]; methodOk {
		if route, routeOk := datadogTags[attributeHTTPRoute]; routeOk {
			return fmt.Sprintf("%s %s", method, route)
		}

		if grpcRoute, grpcRouteOk := datadogTags[grpcPath]; grpcRouteOk {
			return fmt.Sprintf("%s %s", method, grpcRoute)
		}

		return method
	}

	//add resource conventions for messaging queues, operaton + destination
	if msgOperation, msgOperationOk := datadogTags[attributeMessagingOperation]; msgOperationOk {
		if destination, destinationOk := datadogTags[attributeMessagingDestination]; destinationOk {
			return fmt.Sprintf("%s %s", msgOperation, destination)
		}

		return msgOperation
	}

	// add resource convention for rpc services , method+service, fallback to just method if no service attribute
	if rpcMethod, rpcMethodOk := datadogTags[attributeRPCMethod]; rpcMethodOk {
		if rpcService, rpcServiceOk := datadogTags[attributeRPCService]; rpcServiceOk {
			return fmt.Sprintf("%s %s", rpcMethod, rpcService)
		}

		return rpcMethod
	}

	return fmt.Sprintf("%s", s["name"].(string))
}

// TODO: some clients send SPAN_KIND_UNSPECIFIED for valid kinds
// we also need a more formal mapping for cache and db types
func spanKindToDatadogType(kind string) string {
	switch kind {
	case "CLIENT":
		return httpKind
	case "SERVER":
		return webKind
	default:
		return customKind
	}
}

func getSpanErrorAndSetTags(s map[string]interface{}, tags map[string]string) int32 {
	var isError int32
	// Set Span Status and any response or error details
	status := s["status"].(map[string]interface{})
	code := int32(status["code"].(float64))

	switch code {
	case otelOkCode:
		isError = okCode
	case otelErrorCode:
		isError = errorCode
	default:
		isError = okCode
	}

	if isError == errorCode {
		extractErrorTagsFromEvents(s, tags)
		// If we weren't able to pull an error type or message, go ahead and set
		// these to the old defaults
		if _, ok := tags[tagErrorType]; !ok {
			tags[tagErrorType] = "ERR_CODE_" + strconv.FormatInt(int64(code), 10)
		}

		if _, ok := tags[tagErrorMsg]; !ok {
			if msg, ok := status["message"]; ok {
				msgString := msg.(string)
				tags[tagErrorMsg] = msgString
			} else {
				tags[tagErrorMsg] = "ERR_CODE_" +  strconv.FormatInt(int64(code), 10)
			}
		}
	}

	// if status code exists check if error depending on type
	if tags[attributeHTTPStatusCode] != "" {
		httpStatusCode, err := strconv.ParseInt(tags[attributeHTTPStatusCode], 10, 64)
		if err == nil {
			// for 500 type, always mark as error
			if httpStatusCode >= 500 {
				isError = errorCode
				// for 400 type, mark as error if it is an http client
			} else if NormalizeSpanKind(int32(s["kind"].(float64))) == "CLIENT" && httpStatusCode >= 400 {
				isError = errorCode
			}
		}
	}

	return isError
}

// Finds the last exception event in the span, and surfaces it to DataDog. DataDog spans only support a single
// exception per span, but otel supports many exceptions as "Events" on a given span. The last exception was
// chosen for now as most otel-instrumented libraries (http, pg, etc.) only capture a single exception (if any)
// per span. If multiple exceptions are logged, it's my assumption that the last exception is most likely the
// exception that escaped the scope of the span.
//
// TODO:
//  Seems that the spec has an attribute that hasn't made it to the collector yet -- "exception.escaped".
//  This seems optional (SHOULD vs. MUST be set), but it's likely that we want to bubble up the exception
//  that escaped the scope of the span ("exception.escaped" == true) instead of the last exception event
//  in the case that these events differ.
//
//  https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/exceptions.md#attributes
func extractErrorTagsFromEvents(s map[string]interface{}, tags map[string]string) {
	evts := s["events"].([]map[string]interface{})
	for i := len(evts) - 1; i >= 0; i-- {
		evt := evts[i]

		evtNameFormatted := evt["name"].(string)
		if evtNameFormatted == attributeExceptionEventName {
			attribs := evt["attributes"].(map[string]string)
			if errType, ok := attribs[attributeExceptionType]; ok {
				tags[tagErrorType] = errType
			}
			if errMsg, ok := attribs[attributeExceptionMessage]; ok {
				tags[tagErrorMsg] = errMsg
			}
			if errStack, ok := attribs[attributeExceptionStacktrace]; ok {
				tags[tagErrorStack] = errStack
			}
			return
		}		
	}
}

// Convert Span Events to a string so that they can be appended to the span as a tag.
// Span events are probably better served as Structured Logs sent to the logs API
// with the trace id and span id added for log/trace correlation. However this would
// mean a separate API intake endpoint and also Logs and Traces may not be enabled for
// a user, so for now just surfacing this information as a string is better than not
// including it at all. The tradeoff is that this increases the size of the span and the
// span may have a tag that exceeds max size allowed in backend/ui/etc.
//
// TODO: Expose configuration option for collecting Span Events as Logs within Datadog
// and add forwarding to Logs API intake.
// func eventsToString(evts pdata.SpanEventSlice) string {
// 	eventArray := make([]map[string]interface{}, 0, evts.Len())
// 	for i := 0; i < evts.Len(); i++ {
// 		spanEvent := evts.At(i)
// 		event := map[string]interface{}{}
// 		event[eventNameTag] = spanEvent.Name()
// 		event[eventTimeTag] = spanEvent.Timestamp()
// 		event[eventAttrTag] = tracetranslator.AttributeMapToMap(spanEvent.Attributes())
// 		eventArray = append(eventArray, event)
// 	}
// 	eventArrayBytes, _ := json.Marshal(&eventArray)
// 	return string(eventArrayBytes)
// }
