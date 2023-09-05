/*
Copyright 2023 KubeAGI.

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

package evaluator

const DefaultOutputFormat = `json
{"rating": "xxx","assesement": "xxx"}
`

const (
	DefaultPromptTemplate = `
I have a component in my repository that I'm interested in assessing the {{ .Dimension }} implications of this component.Below are some context from component {{ .Dimension }} check:
1. A component is a software which can be deployed into kubernetes with its helm charts.
2. Use customized tekton pipelines to run the tests.

Below are the tested Tasks:
  
{{ .Tasks }}


**Component {{ .Dimension }} Assessment**:
 - Could you please review the above tested tasks and inform me any potential {{ .Dimension }} risks if I adopt this component in production?

**Final Rating based on above tested tasks**:
 - On a scale of 1 to 10, with 10 being the most reliable, how would you rate the {{ .Dimension }} of this component?

Your answer should follow the below format:
{{.OutputFormat}}
`
)
