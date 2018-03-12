package azure

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
)

func TestBatchBashGenerator(t *testing.T) {
	template := template.New("run.sh.tmpl").Option("missingkey=error").Funcs(template.FuncMap{
		"getLaunchCommand": getLaunchCommand,
	})

	template, err := template.Parse(azureBatchPodTemplate)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	templateVars := BatchPodComponents{
		Containers: []v1.Container{
			v1.Container{
				Name:  "testName",
				Image: "busybox",
				Env: []v1.EnvVar{
					v1.EnvVar{
						Name:  "1",
						Value: "1",
					},
					v1.EnvVar{
						Name:  "2",
						Value: "2",
					},
				},
				Command: []string{
					"sleep",
				},
				Args: []string{
					`15`,
				},
			},
		},
		PullCredentials: []aci.ImageRegistryCredential{
			aci.ImageRegistryCredential{
				Username: "lawrencegripper",
				Password: "testing",
				Server:   "server",
			},
		},
		PodName: "Barry",
		TaskID:  "barryid",
	}

	var output bytes.Buffer
	err = template.Execute(&output, templateVars)
	t.Log(output.String())
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

}
