// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package api

import (
	// "encoding/hex"
	// "encoding/json"
	// "fmt"
	// "strconv"
	// "strings"
	// "time"

	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"

	// "github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	// "go.opentelemetry.io/collector/consumer/pdata"
	// "go.opentelemetry.io/collector/translator/conventions"
	// tracetranslator "go.opentelemetry.io/collector/translator/trace"
	// "go.uber.org/zap"
	// "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	// "gopkg.in/zorkian/go-datadog-api.v2"

	// "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	// "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
	// "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metrics"
	// "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils"
)

const (
	attributeDeploymentEnvironment    string = "deployment.environment"
	attributeDatadogEnvironment       string = "env"
	attributeServiceName              string = "service.name"
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
	resourceNoServiceName             string = "OTLPResourceNoServiceName"
	// tagContainersTags specifies the name of the tag which holds key/value
	// pairs representing information about the container (Docker, EC2, etc).
	tagContainersTags = "_dd.tags.container"

)

// converts a Trace's resource spans into a trace payload
func OtelResourceSpansToDatadogSpans(rs map[string]interface{}) error {
	// get env tag
	// env := cfg.Env

	log.Errorf("starti")
	traces := []*pb.Span{}

	resourceraw := rs["resource"]
	ilsraw := rs["instrumentationLibrarySpans"]


	log.Errorf("neext")
	// payload := pb.TracePayload{
	// 	HostName:     hostname,
	// 	Env:          env,
	// 	Traces:       []*pb.APITrace{},
	// 	Transactions: []*pb.Span{},
	// }

	// if resource.Attributes().Len() == 0 && ilss.Len() == 0 {
	// 	return payload
	// }

	if spans, ilok := ilsraw.([]interface{}); ilok {
		if resource, rok := resourceraw.(map[string]interface{}); rok {
			// log.Errorf("reszource isz %s", resource["attributes"])

			resourceServiceName, datadogTags := resourceToDatadogServiceNameAndAttributeMap(resource)
			extractDatadogEnv(datadogTags)

			log.Errorf("translated resource attr : %s and %s", resourceServiceName, datadogTags)

			if attributes, aok := resource["attributes"].([]interface{}); aok {
				if len(attributes) == 0 && len(spans) == 0 {

					log.Errorf("nothing %s", traces)
				} else {

					log.Errorf("else")
					if len(spans) > 0 {
						for i := 0; i < len(spans); i++ { 
							ilspan := spans[i].(map[string]interface{})
							// get IL info

							// extractInstrumentationLibraryTags(ilspan.InstrumentationLibrary(), datadogTags)
							il := ilspan["instrumentationLibrary"].(map[string]interface{})
							extractInstrumentationLibraryTags(il, datadogTags)

							log.Errorf("il lib %s %s", il["name"], il["version"] )
// 2021-04-10 16:43:03 EDT | TRACE | ERROR | (pkg/trace/api/otel.go:105 in OtelResourceSpansToDatadogSpans) | il lib @opentelemetry/plugin-express 0.13.1
							// log.Errorf("translated ilspans : %s", ilspan)

							ilSpanList = ilspan["spans"].([]interface{})

							for j := 0; j < len(spans); j++ { 
								span = ilSpanList[i].map(map[string]interface{})
								spanToDatadogSpan(span, resourceServiceName, datadogTags)
							}

							// log.Errorf("translated resource attr : %s", attributes)

							// TODO:
							// get a service name tag and an tags hash seeded with resource attributes
							// get environment name tag from tags 
							// get config as falllback
							// loop over instrumentationn library span batch:
							//   - extract instrumentationn library name and version from json and apply to tags
							//   -  loop over span batch:
							// 		- convert span to datadog span, passing in tags and config
							//			- fill in details and translation but broadly:
							// 				- trace_id, span_id, parent_span_id, start+duration (nano?), meta, metrics, status, tags, err tags, resource, op name, kind/type , unified service tags...(?)
							// 		- add to trace map 
							// export trace map as pb.Traces []*Trace
						}


					} else {
						// return traces
						log.Errorf("nada")
					}

				}
			} else {
				log.Errorf("na 3rd")
			}
		} else {
			log.Errorf("na 2nd")
		}
	} else {
		log.Errorf("na 1st")
	}

	return nil
}


func resourceToDatadogServiceNameAndAttributeMap(
	resource map[string]interface{},
) (serviceName string, datadogTags map[string]string) {
	// attrs := resource.Attributes()
	log.Errorf("ok and %s", resource["attributes"])
	if attributes, aok := resource["attributes"].([]interface{}); aok {
		attrs := attributes
		
		log.Errorf("atttrs %s", attributes)
		// predefine capacity where possible with extra for _dd.tags.container payload		
		datadogTags = make(map[string]string, len(attrs) + 1)

		if len(attrs) == 0 {
			return resourceNoServiceName, datadogTags
		}

		for i := 0; i < len(attrs); i++ { 
			v := attrs[i].(map[string]interface{})
			// log.Errorf("na %s", v)
			key := v["key"].(string)

			if valueMap, vok := v["value"].(map[string]interface{}); vok {
				value := valueMap["stringValue"].(string)
				datadogTags[key] = value
				log.Errorf("na %s", datadogTags)
			}
		}
		// attrs.ForEach(func(k string, v map[stringValue]string) {
		// 	log.Errorf("na %s", v)
		// 	// datadogTags[k] = tracetranslator.AttributeValueToString(v, false)
		// })
		
		serviceName = extractDatadogServiceName(datadogTags)

		return serviceName, datadogTags
	} else {
		log.Errorf("no buenoo")
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
		datadogTags[conventions.InstrumentationLibraryName] = ilName
	}
	if ilVer := il["version"]; ilVer != "" {
		datadogTags[conventions.InstrumentationLibraryVersion] = ilVer
	}
}

// convertSpan takes an internal span representation and returns a Datadog span.
func spanToDatadogSpan(span, s map[string]interface{},
	serviceName string,
	datadogTags map[string]string,
) *pb.Span {

	tags := aggregateSpanTags(s, datadogTags)

	// otel specification resource service.name takes precedence
	// and configuration DD_ENV as fallback if it exists
	if cfg.Service != "" {
		// prefer the collector level service name over an empty string or otel default
		if serviceName == "" || serviceName == tracetranslator.ResourceNoServiceName {
			serviceName = cfg.Service
		}
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)

	//  canonical resource attribute version should override others if it exists
	if rsTagVersion := tags[conventions.AttributeServiceVersion]; rsTagVersion != "" {
		tags[versionTag] = rsTagVersion
	} else {
		// if no version tag exists, set it if provided via config
		if cfg.Version != "" {
			if tagVersion := tags[versionTag]; tagVersion == "" {
				tags[versionTag] = cfg.Version
			}
		}
	}

	// get tracestate as just a general tag
	if len(s.TraceState()) > 0 {
		tags[tracetranslator.TagW3CTraceState] = string(s.TraceState())
	}

	// get events as just a general tag
	if s.Events().Len() > 0 {
		tags[eventsTag] = eventsToString(s.Events())
	}

	// get start/end time to calc duration
	startTime := s.StartTime()
	endTime := s.EndTime()
	duration := int64(endTime) - int64(startTime)

	// it's possible end time is unset, so default to 0 rather than using a negative number
	if s.EndTime() == 0 {
		duration = 0
	}

	// by checking for error and setting error tags before creating datadog span
	// we can then set Error field when creating and predefine a max meta capacity
	isSpanError := getSpanErrorAndSetTags(s, tags)

	span := &pb.Span{
		TraceID:  decodeAPMTraceID(s.TraceID().Bytes()),
		SpanID:   decodeAPMSpanID(s.SpanID().Bytes()),
		Name:     getDatadogSpanName(s, tags),
		Resource: getDatadogResourceName(s, tags),
		Service:  normalizedServiceName,
		Start:    int64(startTime),
		Duration: duration,
		Metrics:  map[string]float64{},
		Meta:     make(map[string]string, len(tags)),
		Type:     spanKindToDatadogType(s.Kind()),
		Error:    isSpanError,
	}

	if !s.ParentSpanID().IsEmpty() {
		span.ParentID = decodeAPMSpanID(s.ParentSpanID().Bytes())
	}

	// Set Attributes as Tags
	for key, val := range tags {
		setStringTag(span, key, val)
	}

	return span
}

func aggregateSpanTags(span map[string]interface{}, datadogTags map[string]string) map[string]string {
	// predefine capacity as at most the size attributes and global tags
	// there may be overlap between the two.

	spanAttributes = span["attributes"].([]interface{})
	spanTags := make(map[string]string, len(spanAttributes) + len(datadogTags))

	for key, val := range datadogTags {
		spanTags[key] = val
	}

	for i := 0; i < len(spanAttributes); i++ { 
		attributeMap := spanAttributes[i].(map[string]interface{})
		// log.Errorf("na %s", v)
		key := attributeMap["key"].(string)

		if valueMap, vok := attributeMap["value"].(map[string]interface{}); vok {
			value := valueMap["stringValue"].(string)
			spanTags[key] = value
			log.Errorf("na %s", datadogTags)
		}
	}


	// TODO: figure this stuff out, does this get done later?
	spanTags[tagContainersTags] = buildDatadogContainerTags(spanTags)
	return spanTags
}

// // buildDatadogContainerTags returns container and orchestrator tags belonging to containerID
// // as a comma delimeted list for datadog's special container tag key
// func buildDatadogContainerTags(spanTags map[string]string) string {
// 	var b strings.Builder

// 	if val, ok := spanTags[conventions.AttributeContainerID]; ok {
// 		b.WriteString(fmt.Sprintf("%s:%s,", "container_id", val))
// 	}
// 	if val, ok := spanTags[conventions.AttributeK8sPod]; ok {
// 		b.WriteString(fmt.Sprintf("%s:%s,", "pod_name", val))
// 	}

// 	return strings.TrimSuffix(b.String(), ",")
// }