// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

// +build !dogstatsd

package tagger

import (
	"github.com/DataDog/datadog-agent/pkg/tagger/collectors"
	"github.com/DataDog/datadog-agent/pkg/tagger/local"
	"github.com/DataDog/datadog-agent/pkg/tagger/remote"
	"github.com/DataDog/datadog-agent/pkg/util/flavor"
)

func init() {
	if flavor.GetFlavor() == flavor.DefaultAgent {
		defaultTagger = local.NewTagger(collectors.DefaultCatalog)
	} else {
		defaultTagger = remote.NewTagger()
	}
}
