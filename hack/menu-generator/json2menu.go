/*
Copyright 2023 The Kubebb Authors.

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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kubebb/core/api/t7d.io/v1beta1"

	"github.com/TylerBrock/colorjson"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/mitchellh/mapstructure"
	"k8s.io/klog/v2"
)

func jsonArrayParse(vv []interface{}) []v1beta1.Menu {
	menus := make([]v1beta1.Menu, 0)
	for _, u := range vv {
		switch vv1 := u.(type) {
		case string, float64, uint32, int32, bool, []string:
			return nil
		case []interface{}:
			menus = jsonArrayParse(vv1)
		case interface{}:
			m1 := u.(map[string]interface{})
			menus = append(menus, jsonObjectParse(m1))
		default:
			//fmt.Println("  ", i, "[type?_]", u, ", ", vv1)
		}
	}
	return menus
}

func jsonObjectParse(f interface{}) v1beta1.Menu {
	cf := colorjson.NewFormatter()
	cf.Indent = 4
	m := f.(map[string]interface{})
	curLevel := map[string]interface{}{}
	curMenu := v1beta1.Menu{}
	subMenus := make([]v1beta1.Menu, 0)
	for k, v := range m {
		switch vv := v.(type) {
		case string, float64, uint32, int32, bool, []string:
			curLevel[k] = v
		case nil:
		case []interface{}:
			menuList := jsonArrayParse(vv)
			if len(menuList) != 0 {
				subMenus = append(subMenus, menuList...)
			} else {
				curLevel[k] = v
			}
		case interface{}:
			m1 := v.(map[string]interface{})
			menu := jsonObjectParse(m1)
			if menu.Name != "" {
				subMenus = append(subMenus, menu)
			} else {
				curLevel[k] = v
			}
		default:
			//	fmt.Println(k, "[type?]", vv)
		}
	}
	if err := mapstructure.Decode(curLevel["annotations"], &curMenu.Annotations); err != nil {
		klog.Error(err)
	}
	if err := mapstructure.Decode(curLevel["labels"], &curMenu.Labels); err != nil {
		klog.Error(err)
	}
	if err := mapstructure.Decode(curLevel, &curMenu.Spec); err != nil {
		klog.Error(err)
		return curMenu
	}
	curMenu.TypeMeta = metav1.TypeMeta{Kind: "Menu", APIVersion: v1beta1.GroupVersion.String()}
	curMenu.ObjectMeta.Name = curMenu.Spec.Id
	curMenu.Spec.Id = ""
	if len(subMenus) != 0 {
		False := false
		for _, menu := range subMenus {
			menu.Spec.ParentOwnerReferences = metav1.OwnerReference{
				Kind:               "Menu",
				APIVersion:         v1beta1.GroupVersion.String(),
				Name:               curMenu.Name,
				Controller:         &False,
				BlockOwnerDeletion: &False,
			}
			menu.Spec.Parent = ""
			if HelmPostHook {
				helmAnnotation(&menu)
			}
			b, _ := yaml.Marshal(menu)
			fmt.Println(string(b))
			fmt.Println("---")
		}
	}

	return curMenu
}

var InputPath, outputPath string
var HelmPostHook bool

func init() {
	rootCmd.PersistentFlags().StringVar(&InputPath, "from", "", "the input path of the raw json menu data")
	rootCmd.PersistentFlags().BoolVar(&HelmPostHook, "hook", false, "add post-install and post-upgrade hook in menu  annotations.")
}

func main() {
	Execute()
}

var rootCmd = &cobra.Command{
	Use:   "menu-generator",
	Short: "menu-genenrator is a tools that converts menu json to menu CR",
	Long:  `a tools to help convert statis menu into kubernetes menu CRD`,
	Run: func(cmd *cobra.Command, args []string) {
		menuGenerate(InputPath, outputPath)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
func menuGenerate(input, output string) {
	False := false
	menuByte, err := os.ReadFile(InputPath)
	if err != nil {
		klog.Error(err)
		return
	}

	if strings.Index(string(menuByte[:]), "[") == 0 {
		var fs []interface{}
		err := json.Unmarshal(menuByte, &fs)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for _, f := range fs {
			topMenu := jsonObjectParse(f)
			if topMenu.Spec.Parent != "" {
				topMenu.Spec.ParentOwnerReferences = metav1.OwnerReference{
					Kind:               "Menu",
					APIVersion:         v1beta1.GroupVersion.String(),
					Name:               topMenu.Spec.Parent,
					Controller:         &False,
					BlockOwnerDeletion: &False,
				}
				topMenu.Spec.Parent = ""
			}
			if HelmPostHook {
				helmAnnotation(&topMenu)
			}
			b, _ := yaml.Marshal(topMenu)
			//b, _ := json.MarshalIndent(topMenu, "", "  ")
			fmt.Println(string(b))
			fmt.Println("---")
		}
	} else {
		var f interface{}
		if err := json.Unmarshal(menuByte, &f); err != nil {
			klog.Error(err)
			return
		}
		topMenu := jsonObjectParse(f)
		if topMenu.Spec.Parent != "" {
			topMenu.Spec.ParentOwnerReferences = metav1.OwnerReference{
				Kind:               "Menu",
				APIVersion:         v1beta1.GroupVersion.String(),
				Name:               topMenu.Spec.Parent,
				Controller:         &False,
				BlockOwnerDeletion: &False,
			}
			topMenu.Spec.Parent = ""
		}
		if HelmPostHook {
			helmAnnotation(&topMenu)
		}
		b, _ := yaml.Marshal(topMenu)
		//b, _ := json.MarshalIndent(topMenu, "", "  ")
		fmt.Println(string(b))
	}
}

func helmAnnotation(menu *v1beta1.Menu) {
	if menu.Annotations == nil {
		menu.Annotations = map[string]string{}
	}
	menu.Annotations["helm.sh/hook"] = "post-install,post-upgrade"
	menu.Annotations["helm.sh/hook-weight"] = "-6"
}
