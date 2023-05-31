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

package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// TODO Should we compress the data into a configmap?
// helm compresses the release because the user does not access the data directly, but through the helm command line tool.
// Compression reduces the size, but is not intuitive
// inspire by https://github.com/helm/helm/blob/2398830f183b6d569224ae693ae9215fed5d1372/pkg/storage/driver/util.go
var b64 = base64.StdEncoding // nolint

var magicGzip = []byte{0x1f, 0x8b, 0x08}

// GzipData encodes data returning a gzipped []byte representation, or error.
func GzipData(data string) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err = w.Write([]byte(data)); err != nil {
		return nil, err
	}
	defer w.Close()
	return buf.Bytes(), nil
}

// Decode decodes the bytes of data into string slice
// Data must contain a base64 encoded gzipped string of string slice, otherwise an error is returned.
func Decode(b []byte) (res []string, err error) {
	if len(b) <= 3 {
		return nil, fmt.Errorf("data too short")
	}
	if !bytes.Equal(b[0:3], magicGzip) {
		return nil, fmt.Errorf("not a gzipped data")
	}
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	d, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(d, &res); err != nil {
		return nil, err
	}
	return res, nil
}
