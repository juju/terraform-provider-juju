// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package testsing

import (
	"bytes"
	"text/template"
)

type TemplateData map[string]string

// GetStringFromTemplateWithData returns a string from a template with data.
//
// Here is the example usage:
// templateStr := GetStringFromTemplateWithData(
//
//	"testAccResourceApplicationWithRevisionAndConfig",
//	`
//
//	resource "juju_model" "this" {
//	 name = "{{.ModelName}}"
//	}
//
//	resource "juju_application" "{{.AppName}}" {
//	 name  = "{{.AppName}}"
//	 model = juju_model.this.name
//
//	 charm {
//	   name     = "{{.AppName}}"
//	   revision = {{.Revision}}
//	   channel  = "latest/edge"
//	 }
//
//	 {{ if ne .ConfigParamName "" }}
//	 config = {
//	   {{.ConfigParamName}} = "{{.ConfigParamName}}-value"
//	 }
//	 {{ end }}
//
//	 units = 1
//	}
//
//	`, utils.TemplateData{
//					"ModelName":       "test-model",
//					"AppName":         "test-app"
//					"Revision":        fmt.Sprintf("%d", 7),
//					"ConfigParamName": "test-config-param-name",
//				})
//
// The templateStr will be:
//
//	resource "juju_model" "this" {
//	  name = "test-model"
//	}
//
//	resource "juju_application" "test-app" {
//	  name  = "test-app"
//	  model = juju_model.this.name
//
//	  charm {
//	    name     = "test-app"
//	    revision = 7
//	    channel  = "latest/edge"
//	  }
//
//	  config = {
//	    test-config-param-name = "test-config-param-name-value"
//	  }
//
//	  units = 1
//	}
func GetStringFromTemplateWithData(templateName string, templateStr string, templateData TemplateData) string {
	tmpl, err := template.New(templateName).Parse(templateStr)
	if err != nil {
		panic(err)
	}
	var tpl bytes.Buffer
	err = tmpl.Execute(&tpl, templateData)
	if err != nil {
		panic(err)
	}
	return tpl.String()
}
