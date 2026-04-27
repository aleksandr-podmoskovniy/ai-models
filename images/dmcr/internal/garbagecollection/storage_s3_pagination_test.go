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
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestS3PrefixStoreObjectPaginationUsesContinuationToken(t *testing.T) {
	t.Parallel()

	store, transport := newS3PrefixStoreForTest(t,
		func(request *http.Request) (*http.Response, error) {
			assertQueryValue(t, request, "prefix", "prefix")
			assertQueryValue(t, request, "continuation-token", "")
			return s3XMLResponse(http.StatusOK, listObjectsV2XML(true, "next-page", "prefix/a")), nil
		},
		func(request *http.Request) (*http.Response, error) {
			assertQueryValue(t, request, "prefix", "prefix")
			assertQueryValue(t, request, "continuation-token", "next-page")
			return s3XMLResponse(http.StatusOK, listObjectsV2XML(false, "", "prefix/b")), nil
		},
	)

	var keys []string
	err := store.ForEachObjectInfo(context.Background(), "/prefix/", func(info prefixObjectInfo) {
		keys = append(keys, info.Key)
	})

	assertNoError(t, err)
	transport.assertComplete()
	assertStringSlice(t, keys, []string{"prefix/a", "prefix/b"})
}

func TestS3PrefixStoreObjectPaginationRejectsMissingNextCursor(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t, func(*http.Request) (*http.Response, error) {
		return s3XMLResponse(http.StatusOK, listObjectsV2XML(true, "", "prefix/a")), nil
	})

	err := store.ForEachObjectInfo(context.Background(), "prefix", func(prefixObjectInfo) {})

	assertErrorContains(t, err, "without next cursor")
}

func TestS3PrefixStoreObjectPaginationRejectsRepeatedContinuationToken(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t,
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listObjectsV2XML(true, "same-token", "prefix/a")), nil
		},
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listObjectsV2XML(true, "same-token", "prefix/b")), nil
		},
	)

	err := store.ForEachObjectInfo(context.Background(), "prefix", func(prefixObjectInfo) {})

	assertErrorContains(t, err, "repeated cursor")
}

func TestS3PrefixStoreMultipartUploadPaginationUsesMarkers(t *testing.T) {
	t.Parallel()

	store, transport := newS3PrefixStoreForTest(t,
		func(request *http.Request) (*http.Response, error) {
			assertQueryValue(t, request, "prefix", "prefix")
			assertQueryValue(t, request, "key-marker", "")
			assertQueryValue(t, request, "upload-id-marker", "")
			return s3XMLResponse(http.StatusOK, listMultipartUploadsXML(true, "prefix/a", "upload-1", multipartUploadXML("prefix/a", "upload-1"))), nil
		},
		func(request *http.Request) (*http.Response, error) {
			assertQueryValue(t, request, "prefix", "prefix")
			assertQueryValue(t, request, "key-marker", "prefix/a")
			assertQueryValue(t, request, "upload-id-marker", "upload-1")
			return s3XMLResponse(http.StatusOK, listMultipartUploadsXML(false, "", "", multipartUploadXML("prefix/b", "upload-2"))), nil
		},
	)

	var uploads []multipartUploadInfo
	err := store.ForEachMultipartUpload(context.Background(), "prefix", func(info multipartUploadInfo) {
		uploads = append(uploads, info)
	})

	assertNoError(t, err)
	transport.assertComplete()
	if got, want := len(uploads), 2; got != want {
		t.Fatalf("upload count = %d, want %d", got, want)
	}
	if got, want := uploads[1].UploadID, "upload-2"; got != want {
		t.Fatalf("upload[1] ID = %q, want %q", got, want)
	}
}

func TestS3PrefixStoreMultipartUploadPaginationRejectsRepeatedMarker(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t,
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listMultipartUploadsXML(true, "prefix/a", "upload-1", multipartUploadXML("prefix/a", "upload-1"))), nil
		},
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listMultipartUploadsXML(true, "prefix/a", "upload-1", multipartUploadXML("prefix/b", "upload-2"))), nil
		},
	)

	err := store.ForEachMultipartUpload(context.Background(), "prefix", func(multipartUploadInfo) {})

	assertErrorContains(t, err, "repeated cursor")
}

func TestS3PrefixStoreMultipartUploadPaginationRejectsMissingUploadMarkerForSameKey(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t,
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listMultipartUploadsXML(true, "prefix/a", "upload-1", multipartUploadXML("prefix/a", "upload-1"))), nil
		},
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listMultipartUploadsXML(true, "prefix/a", "", multipartUploadXML("prefix/a", "upload-2"))), nil
		},
	)

	err := store.ForEachMultipartUpload(context.Background(), "prefix", func(multipartUploadInfo) {})

	assertErrorContains(t, err, "without upload cursor")
}

func TestS3PrefixStorePartPaginationUsesPartNumberMarker(t *testing.T) {
	t.Parallel()

	store, transport := newS3PrefixStoreForTest(t,
		func(request *http.Request) (*http.Response, error) {
			assertQueryValue(t, request, "uploadId", "upload-1")
			assertQueryValue(t, request, "part-number-marker", "")
			return s3XMLResponse(http.StatusOK, listPartsXML(true, "2", 1, 2)), nil
		},
		func(request *http.Request) (*http.Response, error) {
			assertQueryValue(t, request, "uploadId", "upload-1")
			assertQueryValue(t, request, "part-number-marker", "2")
			return s3XMLResponse(http.StatusOK, listPartsXML(false, "", 3)), nil
		},
	)

	count, err := store.CountMultipartUploadParts(context.Background(), "prefix/a", "upload-1")

	assertNoError(t, err)
	transport.assertComplete()
	if count != 3 {
		t.Fatalf("part count = %d, want 3", count)
	}
}

func TestS3PrefixStorePartPaginationRejectsRepeatedMarker(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t,
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listPartsXML(true, "2", 1)), nil
		},
		func(*http.Request) (*http.Response, error) {
			return s3XMLResponse(http.StatusOK, listPartsXML(true, "2", 2)), nil
		},
	)

	_, err := store.CountMultipartUploadParts(context.Background(), "prefix/a", "upload-1")

	assertErrorContains(t, err, "repeated cursor")
}

func TestS3PrefixStorePartPaginationRejectsMissingNextMarker(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t, func(*http.Request) (*http.Response, error) {
		return s3XMLResponse(http.StatusOK, listPartsXML(true, "", 1)), nil
	})

	_, err := store.CountMultipartUploadParts(context.Background(), "prefix/a", "upload-1")

	assertErrorContains(t, err, "without next cursor")
}

func TestS3PrefixStoreCountMultipartUploadPartsMapsNoSuchUpload(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t, func(*http.Request) (*http.Response, error) {
		return s3XMLResponse(http.StatusNotFound, s3ErrorXML("NoSuchUpload", "gone")), nil
	})

	_, err := store.CountMultipartUploadParts(context.Background(), "prefix/a", "upload-1")

	if !errors.Is(err, errMultipartUploadGone) {
		t.Fatalf("CountMultipartUploadParts() error = %v, want errMultipartUploadGone", err)
	}
}

func TestS3PrefixStoreAbortMultipartUploadIgnoresNoSuchUpload(t *testing.T) {
	t.Parallel()

	store, transport := newS3PrefixStoreForTest(t, func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", request.Method)
		}
		return s3XMLResponse(http.StatusNotFound, s3ErrorXML("NoSuchUpload", "gone")), nil
	})

	err := store.AbortMultipartUpload(context.Background(), "prefix/a", "upload-1")

	assertNoError(t, err)
	transport.assertComplete()
}

func TestS3PrefixStoreDeletePrefixReportsObjectErrors(t *testing.T) {
	t.Parallel()

	store, _ := newS3PrefixStoreForTest(t,
		func(request *http.Request) (*http.Response, error) {
			assertQueryValue(t, request, "prefix", "prefix/session-1/")
			return s3XMLResponse(http.StatusOK, listObjectsV2XML(false, "", "prefix/session-1/a")), nil
		},
		func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", request.Method)
			}
			if _, found := request.URL.Query()["delete"]; !found {
				t.Fatalf("delete query is missing from %q", request.URL.RawQuery)
			}
			return s3XMLResponse(http.StatusOK, deleteObjectsErrorXML("prefix/session-1/a", "AccessDenied", "denied")), nil
		},
	)

	err := store.DeletePrefix(context.Background(), "prefix/session-1/")

	assertErrorContains(t, err, "AccessDenied")
}
