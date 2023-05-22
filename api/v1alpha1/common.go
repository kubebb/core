package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

var (
	// DisplayNameAnnotationKey is the key of the annotation used to set the display name of the resource
	DisplayNameAnnotationKey = GroupVersion.Group + "/displayname"
)

// ComponetVersions Indicates the fields required for a specific version of Component.
type ComponetVersions struct {
	Version    string      `json:"version"`
	AppVersion string      `json:"appVersion"`
	UpdatedAt  metav1.Time `json:"updatedAt"`
	CreatedAt  metav1.Time `json:"createdAt"`
	Digest     string      `json:"digest"`
}

// inspire by https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/pkg/chart/metadata.go#L26
// Maintainer describes a Chart maintainer.
type Maintainer struct {
	// Name is a user name or organization name
	Name string `json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `json:"email,omitempty"`
	// URL is an optional URL to an address for the named maintainer
	URL string `json:"url,omitempty"`
}

// Override defines the override settings for the component
// The value may be single-valued or multi-valued or one file
type Override struct {
	// Name is the name of the override setting
	Name string `json:"name"`
	// Value is the value of the override setting
	// +optional
	Value string `json:"value,omitempty"`
	// File is the file path of the override setting
	// +optional
	File string `json:"file,omitempty"`
	// Values is the values of the override setting
	// +optional
	Values []string `json:"values,omitempty"`
}
