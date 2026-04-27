/*
Copyright 2026 Flant JSC

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

package garbagecollection

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3RoundTripFunc func(*http.Request) (*http.Response, error)

type sequenceS3Transport struct {
	t        *testing.T
	handlers []s3RoundTripFunc
	index    int
}

func newS3PrefixStoreForTest(t *testing.T, handlers ...s3RoundTripFunc) (*s3PrefixStore, *sequenceS3Transport) {
	t.Helper()

	transport := &sequenceS3Transport{t: t, handlers: handlers}
	client := s3.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("access", "secret", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}, func(options *s3.Options) {
		options.UsePathStyle = true
		options.BaseEndpoint = aws.String("http://s3.test")
	})
	return &s3PrefixStore{bucket: "bucket", client: client}, transport
}

func (t *sequenceS3Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.index >= len(t.handlers) {
		t.t.Fatalf("unexpected S3 request %s %s", request.Method, request.URL.String())
	}
	handler := t.handlers[t.index]
	t.index++
	return handler(request)
}

func (t *sequenceS3Transport) assertComplete() {
	t.t.Helper()
	if t.index != len(t.handlers) {
		t.t.Fatalf("S3 request count = %d, want %d", t.index, len(t.handlers))
	}
}

func s3XMLResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func listObjectsV2XML(truncated bool, nextToken string, keys ...string) string {
	var contents strings.Builder
	for _, key := range keys {
		contents.WriteString("<Contents><Key>")
		contents.WriteString(key)
		contents.WriteString("</Key><LastModified>2026-04-27T00:00:00Z</LastModified></Contents>")
	}
	next := ""
	if nextToken != "" {
		next = "<NextContinuationToken>" + nextToken + "</NextContinuationToken>"
	}
	return fmt.Sprintf(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bucket</Name><IsTruncated>%t</IsTruncated>%s%s</ListBucketResult>`, truncated, contents.String(), next)
}

func listMultipartUploadsXML(truncated bool, nextKey, nextUploadID, uploads string) string {
	next := ""
	if nextKey != "" {
		next += "<NextKeyMarker>" + nextKey + "</NextKeyMarker>"
	}
	if nextUploadID != "" {
		next += "<NextUploadIdMarker>" + nextUploadID + "</NextUploadIdMarker>"
	}
	return fmt.Sprintf(`<ListMultipartUploadsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Bucket>bucket</Bucket><IsTruncated>%t</IsTruncated>%s%s</ListMultipartUploadsResult>`, truncated, uploads, next)
}

func multipartUploadXML(key, uploadID string) string {
	return "<Upload><Key>" + key + "</Key><UploadId>" + uploadID + "</UploadId><Initiated>2026-04-27T00:00:00Z</Initiated></Upload>"
}

func listPartsXML(truncated bool, nextMarker string, parts ...int) string {
	var partXML strings.Builder
	for _, partNumber := range parts {
		partXML.WriteString(fmt.Sprintf("<Part><PartNumber>%d</PartNumber><LastModified>2026-04-27T00:00:00Z</LastModified><ETag>etag</ETag><Size>1</Size></Part>", partNumber))
	}
	next := ""
	if nextMarker != "" {
		next = "<NextPartNumberMarker>" + nextMarker + "</NextPartNumberMarker>"
	}
	return fmt.Sprintf(`<ListPartsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Bucket>bucket</Bucket><Key>prefix/a</Key><UploadId>upload-1</UploadId><IsTruncated>%t</IsTruncated>%s%s</ListPartsResult>`, truncated, partXML.String(), next)
}

func s3ErrorXML(code, message string) string {
	return "<Error><Code>" + code + "</Code><Message>" + message + "</Message></Error>"
}

func deleteObjectsErrorXML(key, code, message string) string {
	return "<DeleteResult><Error><Key>" + key + "</Key><Code>" + code + "</Code><Message>" + message + "</Message></Error></DeleteResult>"
}

func assertQueryValue(t *testing.T, request *http.Request, key, want string) {
	t.Helper()
	if got := request.URL.Query().Get(key); got != want {
		t.Fatalf("query %q = %q, want %q in %q", key, got, want, request.URL.RawQuery)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorContains(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Fatalf("error = %v, want containing %q", err, text)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if !equalStringSlices(got, want) {
		t.Fatalf("slice = %#v, want %#v", got, want)
	}
}
