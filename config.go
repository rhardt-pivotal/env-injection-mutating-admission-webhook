/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"github.com/ghodss/yaml"
	"crypto/tls"
	"flag"
	"io/ioutil"
	"strings"
	"k8s.io/klog"
	corev1 "k8s.io/api/core/v1"
)

// Config contains the server (the webhook) cert and key.
type ServerConfig struct {
	CertFile string
	KeyFile  string
	EnvCfgFile string
}

type EnvConfig struct {
	EnvVars  []corev1.EnvVar  `yaml:"envVars"`
}

var envVarNamesLower map[string]bool

func (c *ServerConfig) addFlags() {
	flag.StringVar(&c.CertFile, "tls-cert-file", c.CertFile, ""+
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert).")
	flag.StringVar(&c.KeyFile, "tls-private-key-file", c.KeyFile, ""+
		"File containing the default x509 private key matching --tls-cert-file.")
	flag.StringVar(&c.EnvCfgFile, "env-cfg-file", "/etc/webhook/config/envvarconfig.yaml", "File containing the env vars to be injected.")
}

func configTLS(config ServerConfig) *tls.Config {
	klog.Infof("Loading certs: %v and key: %v", config.CertFile, config.KeyFile)
	sCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		klog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
		// TODO: uses mutual tls after we agree on what cert the apiserver should use.
		// ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}

func configEnvVars() (error) {
	klog.V(2).Infof("Loading env vars from configmap")
	data, err := ioutil.ReadFile(envCfgFile)
	if err != nil {
		return err
	}

	klog.V(2).Infof("data: %v", string(data))

	var cfg EnvConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	klog.V(2).Infof("Env Vars: %v", cfg.EnvVars)

	envVars = cfg.EnvVars

	envVarNamesLower = make(map[string]bool)
	for _, ev := range envVars{
		envVarNamesLower[strings.ToLower(ev.Name)] = true
	}

	klog.V(2).Infof("new env vars: %v", envVars)
	klog.V(2).Infof("new env var names lower: %v", envVarNamesLower)

	return nil

}
