/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"sigs.k8s.io/apiserver-network-proxy/pkg/agent"
	"sigs.k8s.io/apiserver-network-proxy/pkg/util"
	"sigs.k8s.io/apiserver-network-proxy/proto/header"
)

type GrpcProxyAgentOptions struct {
	// Configuration for authenticating with the proxy-server
	AgentCert string
	AgentKey  string
	CaCert    string

	// Configuration for connecting to the proxy-server
	ProxyServerHost string
	ProxyServerPort int
	AlpnProtos      []string

	// Bind address for the health connections.
	HealthServerHost string
	// Port we listen for health connections on.
	HealthServerPort int
	// Bind address for the admin connections.
	AdminBindAddress string
	// Port we listen for admin connections on.
	AdminServerPort int
	// Enables pprof at host:adminPort/debug/pprof.
	EnableProfiling bool
	// If EnableProfiling is true, this enables the lock contention
	// profiling at host:adminPort/debug/pprof/block.
	EnableContentionProfiling bool

	AgentID          string
	AgentIdentifiers string
	SyncInterval     time.Duration
	ProbeInterval    time.Duration
	SyncIntervalCap  time.Duration
	// After a duration of this time if the agent doesn't see any activity it
	// pings the server to see if the transport is still alive.
	KeepaliveTime time.Duration

	// file contains service account authorization token for enabling proxy-server token based authorization
	ServiceAccountTokenPath string

	// This warns if we attempt to push onto a "full" transfer channel.
	// However checking that the transfer channel is full is not safe.
	// It violates our race condition checking. Adding locks around a potentially
	// blocking call has its own problems, so it cannot easily be made race condition safe.
	// The check is an "unlocked" read but is still use at your own peril.
	WarnOnChannelLimit bool

	SyncForever    bool
	XfrChannelSize int

	// Enables updating the server count by counting the number of valid leases
	// matching the selector.
	CountServerLeases bool
	// Namespace where lease objects are managed.
	LeaseNamespace string
	// Labels on which lease objects are managed.
	LeaseLabel string
	// ServerCountSource describes how server counts should be combined.
	ServerCountSource string
	// Path to kubeconfig (used by kubernetes client for lease listing)
	KubeconfigPath string
	// Content type of requests sent to apiserver.
	APIContentType string
}

func (o *GrpcProxyAgentOptions) ClientSetConfig(dialOptions ...grpc.DialOption) *agent.ClientSetConfig {
	return &agent.ClientSetConfig{
		Address:                 net.JoinHostPort(o.ProxyServerHost, strconv.Itoa(o.ProxyServerPort)),
		AgentID:                 o.AgentID,
		AgentIdentifiers:        o.AgentIdentifiers,
		SyncInterval:            o.SyncInterval,
		ProbeInterval:           o.ProbeInterval,
		SyncIntervalCap:         o.SyncIntervalCap,
		DialOptions:             dialOptions,
		ServiceAccountTokenPath: o.ServiceAccountTokenPath,
		WarnOnChannelLimit:      o.WarnOnChannelLimit,
		SyncForever:             o.SyncForever,
		XfrChannelSize:          o.XfrChannelSize,
		ServerCountSource:       o.ServerCountSource,
	}
}

func (o *GrpcProxyAgentOptions) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("proxy-agent", pflag.ContinueOnError)
	flags.StringVar(&o.AgentCert, "agent-cert", o.AgentCert, "If non-empty secure communication with this cert.")
	flags.StringVar(&o.AgentKey, "agent-key", o.AgentKey, "If non-empty secure communication with this key.")
	flags.StringVar(&o.CaCert, "ca-cert", o.CaCert, "If non-empty the CAs we use to validate clients.")
	flags.StringVar(&o.ProxyServerHost, "proxy-server-host", o.ProxyServerHost, "The hostname to use to connect to the proxy-server.")
	flags.IntVar(&o.ProxyServerPort, "proxy-server-port", o.ProxyServerPort, "The port the proxy server is listening on.")
	flags.StringSliceVar(&o.AlpnProtos, "alpn-proto", o.AlpnProtos, "Additional ALPN protocols to be presented when connecting to the server. Useful to distinguish between network proxy and apiserver connections that share the same destination address.")
	flags.StringVar(&o.HealthServerHost, "health-server-host", o.HealthServerHost, "The host address to listen on, without port.")
	flags.IntVar(&o.HealthServerPort, "health-server-port", o.HealthServerPort, "The port the health server is listening on.")
	flags.IntVar(&o.AdminServerPort, "admin-server-port", o.AdminServerPort, "The port the admin server is listening on.")
	flags.StringVar(&o.AdminBindAddress, "admin-bind-address", o.AdminBindAddress, "Bind address for admin connections. If empty, we will bind to all interfaces.")
	flags.BoolVar(&o.EnableProfiling, "enable-profiling", o.EnableProfiling, "enable pprof at host:admin-port/debug/pprof")
	flags.BoolVar(&o.EnableContentionProfiling, "enable-contention-profiling", o.EnableContentionProfiling, "enable contention profiling at host:admin-port/debug/pprof/block. \"--enable-profiling\" must also be set.")
	flags.StringVar(&o.AgentID, "agent-id", o.AgentID, "The unique ID of this agent. Can also be set by the 'PROXY_AGENT_ID' environment variable. Default to a generated uuid if not set.")
	flags.DurationVar(&o.SyncInterval, "sync-interval", o.SyncInterval, "The initial interval by which the agent periodically checks if it has connections to all instances of the proxy server.")
	flags.DurationVar(&o.ProbeInterval, "probe-interval", o.ProbeInterval, "The interval by which the agent periodically checks if its connections to the proxy server are ready.")
	flags.DurationVar(&o.SyncIntervalCap, "sync-interval-cap", o.SyncIntervalCap, "The maximum interval for the SyncInterval to back off to when unable to connect to the proxy server")
	flags.DurationVar(&o.KeepaliveTime, "keepalive-time", o.KeepaliveTime, "Time for gRPC agent server keepalive.")
	flags.StringVar(&o.ServiceAccountTokenPath, "service-account-token-path", o.ServiceAccountTokenPath, "If non-empty proxy agent uses this token to prove its identity to the proxy server.")
	flags.StringVar(&o.AgentIdentifiers, "agent-identifiers", o.AgentIdentifiers, "Identifiers of the agent that will be used by the server when choosing agent. N.B. the list of identifiers must be in URL encoded format. e.g.,host=localhost&host=node1.mydomain.com&cidr=127.0.0.1/16&ipv4=1.2.3.4&ipv4=5.6.7.8&ipv6=:::::&default-route=true")
	flags.BoolVar(&o.WarnOnChannelLimit, "warn-on-channel-limit", o.WarnOnChannelLimit, "Turns on a warning if the system is going to push to a full channel. The check involves an unsafe read.")
	flags.BoolVar(&o.SyncForever, "sync-forever", o.SyncForever, "If true, the agent continues syncing, in order to support server count changes.")
	flags.IntVar(&o.XfrChannelSize, "xfr-channel-size", 150, "Set the size of the channel for transferring data between the agent and the proxy server.")
	flags.BoolVar(&o.CountServerLeases, "count-server-leases", o.CountServerLeases, "Enables lease counting system to determine the number of proxy servers to connect to.")
	flags.StringVar(&o.LeaseNamespace, "lease-namespace", o.LeaseNamespace, "Namespace where lease objects are managed.")
	flags.StringVar(&o.LeaseLabel, "lease-label", o.LeaseLabel, "The labels on which the lease objects are managed.")
	flags.StringVar(&o.ServerCountSource, "server-count-source", o.ServerCountSource, "Defines how the server counts from lease and from server responses are combined. Possible values: 'default' to use only one source (server or leases depending on other flags), 'max' to take the larger value.")
	flags.StringVar(&o.KubeconfigPath, "kubeconfig", o.KubeconfigPath, "Path to the kubeconfig file")
	flags.StringVar(&o.APIContentType, "kube-api-content-type", o.APIContentType, "Content type of requests sent to apiserver.")
	return flags
}

func (o *GrpcProxyAgentOptions) Print() {
	klog.V(1).Infof("AgentCert set to %q.\n", o.AgentCert)
	klog.V(1).Infof("AgentKey set to %q.\n", o.AgentKey)
	klog.V(1).Infof("CACert set to %q.\n", o.CaCert)
	klog.V(1).Infof("ProxyServerHost set to %q.\n", o.ProxyServerHost)
	klog.V(1).Infof("ProxyServerPort set to %d.\n", o.ProxyServerPort)
	klog.V(1).Infof("ALPNProtos set to %+s.\n", o.AlpnProtos)
	klog.V(1).Infof("HealthServerHost set to %s\n", o.HealthServerHost)
	klog.V(1).Infof("HealthServerPort set to %d.\n", o.HealthServerPort)
	klog.V(1).Infof("Admin bind address set to %q.\n", o.AdminBindAddress)
	klog.V(1).Infof("AdminServerPort set to %d.\n", o.AdminServerPort)
	klog.V(1).Infof("EnableProfiling set to %v.\n", o.EnableProfiling)
	klog.V(1).Infof("EnableContentionProfiling set to %v.\n", o.EnableContentionProfiling)
	klog.V(1).Infof("AgentID set to %s.\n", o.AgentID)
	klog.V(1).Infof("SyncInterval set to %v.\n", o.SyncInterval)
	klog.V(1).Infof("ProbeInterval set to %v.\n", o.ProbeInterval)
	klog.V(1).Infof("SyncIntervalCap set to %v.\n", o.SyncIntervalCap)
	klog.V(1).Infof("Keepalive time set to %v.\n", o.KeepaliveTime)
	klog.V(1).Infof("ServiceAccountTokenPath set to %q.\n", o.ServiceAccountTokenPath)
	klog.V(1).Infof("AgentIdentifiers set to %s.\n", util.PrettyPrintURL(o.AgentIdentifiers))
	klog.V(1).Infof("WarnOnChannelLimit set to %t.\n", o.WarnOnChannelLimit)
	klog.V(1).Infof("SyncForever set to %v.\n", o.SyncForever)
	klog.V(1).Infof("CountServerLeases set to %v.\n", o.CountServerLeases)
	klog.V(1).Infof("LeaseNamespace set to %s.\n", o.LeaseNamespace)
	klog.V(1).Infof("LeaseLabel set to %s.\n", o.LeaseLabel)
	klog.V(1).Infof("ServerCountSource set to %s.\n", o.ServerCountSource)
	klog.V(1).Infof("ChannelSize set to %d.\n", o.XfrChannelSize)
	klog.V(1).Infof("APIContentType set to %v.\n", o.APIContentType)
}

func (o *GrpcProxyAgentOptions) Validate() error {
	if o.AgentKey != "" {
		if _, err := os.Stat(o.AgentKey); os.IsNotExist(err) {
			return fmt.Errorf("error checking agent key %s, got %v", o.AgentKey, err)
		}
		if o.AgentCert == "" {
			return fmt.Errorf("cannot have agent cert empty when agent key is set to \"%s\"", o.AgentKey)
		}
	}
	if o.AgentCert != "" {
		if _, err := os.Stat(o.AgentCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking agent cert %s, got %v", o.AgentCert, err)
		}
		if o.AgentKey == "" {
			return fmt.Errorf("cannot have agent key empty when agent cert is set to \"%s\"", o.AgentCert)
		}
	}
	if o.CaCert != "" {
		if _, err := os.Stat(o.CaCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking agent CA cert %s, got %v", o.CaCert, err)
		}
	}
	if o.ProxyServerPort <= 0 {
		return fmt.Errorf("proxy server port %d must be greater than 0", o.ProxyServerPort)
	}
	if o.HealthServerPort <= 0 {
		return fmt.Errorf("health server port %d must be greater than 0", o.HealthServerPort)
	}
	if o.AdminServerPort <= 0 {
		return fmt.Errorf("admin server port %d must be greater than 0", o.AdminServerPort)
	}
	if o.XfrChannelSize <= 0 {
		return fmt.Errorf("channel size %d must be greater than 0", o.XfrChannelSize)
	}
	if o.EnableContentionProfiling && !o.EnableProfiling {
		return fmt.Errorf("if --enable-contention-profiling is set, --enable-profiling must also be set")
	}
	if o.SyncInterval > o.SyncIntervalCap {
		return fmt.Errorf("sync interval %v must be less than sync interval cap %v", o.SyncInterval, o.SyncIntervalCap)
	}
	if o.ServiceAccountTokenPath != "" {
		if _, err := os.Stat(o.ServiceAccountTokenPath); os.IsNotExist(err) {
			return fmt.Errorf("error checking service account token path %s, got %v", o.ServiceAccountTokenPath, err)
		}
	}
	if err := validateAgentIdentifiers(o.AgentIdentifiers); err != nil {
		return fmt.Errorf("agent address is invalid: %v", err)
	}
	if o.KubeconfigPath != "" {
		if _, err := os.Stat(o.KubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("error checking KubeconfigPath %q, got %v", o.KubeconfigPath, err)
		}
	}
	// Validate labels provided.
	if o.CountServerLeases {
		_, err := util.ParseLabels(o.LeaseLabel)
		if err != nil {
			return err
		}
	}
	if o.ServerCountSource != "" {
		if o.ServerCountSource != "default" && o.ServerCountSource != "max" {
			return fmt.Errorf("--server-count-source must be one of '', 'default', 'max', got %v", o.ServerCountSource)
		}
	}

	return nil
}

func validateAgentIdentifiers(agentIdentifiers string) error {
	decoded, err := url.ParseQuery(agentIdentifiers)
	if err != nil {
		return err
	}
	for idType := range decoded {
		switch header.IdentifierType(idType) {
		case header.IPv4:
		case header.IPv6:
		case header.CIDR:
		case header.Host:
		case header.DefaultRoute:
		default:
			return fmt.Errorf("unknown address type: %s", idType)
		}
	}
	return nil
}

func NewGrpcProxyAgentOptions() *GrpcProxyAgentOptions {
	o := GrpcProxyAgentOptions{
		AgentCert:                 "",
		AgentKey:                  "",
		CaCert:                    "",
		ProxyServerHost:           "127.0.0.1",
		ProxyServerPort:           8091,
		HealthServerHost:          "",
		HealthServerPort:          8093,
		AdminBindAddress:          "127.0.0.1",
		AdminServerPort:           8094,
		EnableProfiling:           false,
		EnableContentionProfiling: false,
		AgentID:                   defaultAgentID(),
		AgentIdentifiers:          "",
		SyncInterval:              1 * time.Second,
		ProbeInterval:             1 * time.Second,
		SyncIntervalCap:           10 * time.Second,
		KeepaliveTime:             1 * time.Hour,
		ServiceAccountTokenPath:   "",
		WarnOnChannelLimit:        false,
		SyncForever:               false,
		XfrChannelSize:            150,
		CountServerLeases:         false,
		LeaseNamespace:            "kube-system",
		LeaseLabel:                "k8s-app=konnectivity-server",
		ServerCountSource:         "default",
		KubeconfigPath:            "",
		APIContentType:            runtime.ContentTypeProtobuf,
	}
	return &o
}

func defaultAgentID() string {
	// Default to the value set by the PROXY_AGENT_ID environment variable. If both the flag &
	// environment variable are set, the flag always wins.
	if id := os.Getenv("PROXY_AGENT_ID"); id != "" {
		return id
	}
	return uuid.New().String()
}
