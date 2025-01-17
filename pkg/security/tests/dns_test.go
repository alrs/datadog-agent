// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build functionaltests
// +build functionaltests

package tests

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/security/ebpf/kernel"
	sprobe "github.com/DataDog/datadog-agent/pkg/security/probe"
	"github.com/DataDog/datadog-agent/pkg/security/secl/rules"
)

func TestDNS(t *testing.T) {
	kv, err := kernel.NewKernelVersion()
	if err != nil {
		t.Fatal(err)
	}

	if kv.IsRH7Kernel() || kv.IsSLES12Kernel() || kv.IsSLES15Kernel() || kv.IsOracleUEKKernel() {
		t.Skip()
	}

	if testEnvironment != DockerEnvironment && !config.IsContainerized() {
		if out, err := loadModule("veth"); err != nil {
			t.Fatalf("couldn't load 'veth' module: %s, %v", string(out), err)
		}
	}

	rule := &rules.RuleDefinition{
		ID:         "test_rule",
		Expression: `dns.question.type == A && dns.question.name == "google.com" && process.file.name == "testsuite"`,
	}

	test, err := newTestModule(t, nil, []*rules.RuleDefinition{rule}, testOpts{enableNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	defer test.Close()

	t.Run("dns", func(t *testing.T) {
		test.WaitSignal(t, func() error {
			_, err = net.LookupIP("google.com")
			if err != nil {
				return err
			}
			return nil
		}, func(event *sprobe.Event, rule *rules.Rule) {
			assert.Equal(t, "dns", event.GetType(), "wrong event type")
			assert.Equal(t, "google.com", event.DNS.Name, "wrong domain name")

			if !validateDNSSchema(t, event) {
				t.Error(event.String())
			}
		})
	})
}
