package drive

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	googleDrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// ---------------------------------------------------------------------------
// Test Helpers
// ---------------------------------------------------------------------------

// newTestService creates a Drive service that talks to a local test server.
func newTestService(t *testing.T, handler http.HandlerFunc) *googleDrive.Service {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	svc, err := googleDrive.NewService(
		context.Background(),
		option.WithEndpoint(srv.URL+"/"),
		option.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	return svc
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func isFilesRequest(r *http.Request) bool {
	return strings.HasSuffix(r.URL.Path, "/files")
}

func isAboutRequest(r *http.Request) bool {
	return strings.HasSuffix(r.URL.Path, "/about")
}

func checkInt64Ptr(t *testing.T, name string, got *int64, want int64) {
	t.Helper()

	if got == nil {
		t.Errorf("%s = nil, want %d", name, want)
		return
	}

	if *got != want {
		t.Errorf("%s = %d, want %d", name, *got, want)
	}
}

// ---------------------------------------------------------------------------
// ListFiles
// ---------------------------------------------------------------------------

func TestListFiles_NilService(t *testing.T) {
	files, err := ListFiles(context.Background(), nil)

	if !errors.Is(err, ErrInvalidService) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidService)
	}

	if files != nil {
		t.Errorf("files = %v, want nil", files)
	}
}

func TestListFiles_Pagination(t *testing.T) {
	var requests int32

	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if !isFilesRequest(r) {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()

		if got := q.Get("q"); got != "trashed = false" {
			t.Errorf("q = %q, want %q", got, "trashed = false")
		}

		if got := q.Get("pageSize"); got != "1000" {
			t.Errorf("pageSize = %q, want 1000", got)
		}

		fields := q.Get("fields")
		if !strings.Contains(fields, "nextPageToken") ||
			!strings.Contains(fields, "files(") {
			t.Errorf("fields = %q, want nextPageToken and files(...)", fields)
		}

		switch n := atomic.AddInt32(&requests, 1); n {
		case 1:
			if got := q.Get("pageToken"); got != "" {
				t.Errorf("first request pageToken = %q, want empty", got)
			}

			writeJSON(w, http.StatusOK, `{
				"nextPageToken": "page-2",
				"files": [{
					"id": "file-1",
					"name": "report.pdf",
					"mimeType": "application/pdf",
					"size": "1234",
					"modifiedTime": "2026-01-02T03:04:05.000Z",
					"webViewLink": "https://drive.google.com/file/d/file-1/view",
					"parents": ["root"],
					"trashed": false
				}]
			}`)

		case 2:
			if got := q.Get("pageToken"); got != "page-2" {
				t.Errorf("second request pageToken = %q, want page-2", got)
			}

			writeJSON(w, http.StatusOK, `{
				"files": [{
					"id": "folder-1",
					"name": "Documents",
					"mimeType": "application/vnd.google-apps.folder",
					"parents": ["root"]
				}]
			}`)

		default:
			t.Errorf("unexpected extra request #%d", n)
			writeJSON(w, http.StatusOK, `{}`)
		}
	})

	files, err := ListFiles(context.Background(), svc)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}

	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}

	first := files[0]

	if first.ID != "file-1" ||
		first.Name != "report.pdf" ||
		first.MimeType != "application/pdf" ||
		first.Size != 1234 ||
		first.ModifiedTime != "2026-01-02T03:04:05.000Z" ||
		first.WebViewLink != "https://drive.google.com/file/d/file-1/view" ||
		first.Trashed {
		t.Errorf("first file mapped incorrectly: %+v", first)
	}

	if !reflect.DeepEqual(first.Parents, []string{"root"}) {
		t.Errorf("first.Parents = %v, want [root]", first.Parents)
	}

	second := files[1]

	if second.ID != "folder-1" ||
		second.Name != "Documents" ||
		second.Size != 0 {
		t.Errorf("second file mapped incorrectly: %+v", second)
	}
}

func TestListFiles_Empty(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{}`)
	})

	files, err := ListFiles(context.Background(), svc)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if files == nil || len(files) != 0 {
		t.Errorf("files = %v, want empty non-nil slice", files)
	}
}

func TestListFiles_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(
			w,
			http.StatusForbidden,
			`{"error":{"code":403,"message":"forbidden"}}`,
		)
	})

	files, err := ListFiles(context.Background(), svc)

	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "failed to list Drive files") {
		t.Errorf("error should be wrapped with context: %v", err)
	}

	if files != nil {
		t.Errorf("files = %v, want nil", files)
	}
}

func TestListFiles_ErrorOnSecondPage(t *testing.T) {
	var requests int32

	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			writeJSON(
				w,
				http.StatusOK,
				`{"nextPageToken":"next","files":[{"id":"a"}]}`,
			)
			return
		}

		writeJSON(
			w,
			http.StatusForbidden,
			`{"error":{"code":403,"message":"forbidden"}}`,
		)
	})

	files, err := ListFiles(context.Background(), svc)

	if err == nil {
		t.Fatal("expected an error")
	}

	if files != nil {
		t.Errorf("partial results must not be returned, got %v", files)
	}
}

func TestListFiles_ContextCanceled(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{}`)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ListFiles(ctx, svc)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// ---------------------------------------------------------------------------
// GetStorageInfo
// ---------------------------------------------------------------------------

func TestGetStorageInfo_NilService(t *testing.T) {
	info, err := GetStorageInfo(context.Background(), nil)

	if !errors.Is(err, ErrInvalidService) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidService)
	}

	if info != nil {
		t.Errorf("info = %+v, want nil", info)
	}
}

func TestGetStorageInfo(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		wantUnlimited bool
		wantLimit     *int64
		wantUsage     int64
		wantDrive     *int64
		wantTrash     *int64
		wantFree      *int64
	}{
		{
			name: "finite quota",
			response: `{
				"storageQuota": {
					"limit": "1000",
					"usage": "300",
					"usageInDrive": "200",
					"usageInDriveTrash": "50"
				}
			}`,
			wantLimit: int64Pointer(1000),
			wantUsage: 300,
			wantDrive: int64Pointer(200),
			wantTrash: int64Pointer(50),
			wantFree:  int64Pointer(700),
		},
		{
			name: "unlimited quota without limit",
			response: `{
				"storageQuota": {
					"usage": "500",
					"usageInDrive": "400",
					"usageInDriveTrash": "50"
				}
			}`,
			wantUnlimited: true,
			wantUsage:     500,
			wantDrive:     int64Pointer(400),
			wantTrash:     int64Pointer(50),
		},
		{
			name: "unlimited quota with zero limit",
			response: `{
				"storageQuota": {
					"limit": "0",
					"usage": "1000"
				}
			}`,
			wantUnlimited: true,
			wantUsage:     1000,
		},
		{
			name: "usage exceeds limit",
			response: `{
				"storageQuota": {
					"limit": "100",
					"usage": "150"
				}
			}`,
			wantLimit: int64Pointer(100),
			wantUsage: 150,
			wantFree:  int64Pointer(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				if !isAboutRequest(r) {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				if got := r.URL.Query().Get("fields"); !strings.Contains(got, "storageQuota") {
					t.Errorf("fields = %q, want storageQuota", got)
				}

				writeJSON(w, http.StatusOK, tt.response)
			})

			info, err := GetStorageInfo(context.Background(), svc)
			if err != nil {
				t.Fatalf("GetStorageInfo: %v", err)
			}

			if info.Unlimited != tt.wantUnlimited {
				t.Errorf(
					"Unlimited = %v, want %v",
					info.Unlimited,
					tt.wantUnlimited,
				)
			}

			if !reflect.DeepEqual(info.Limit, tt.wantLimit) {
				t.Errorf("Limit = %v, want %v", info.Limit, tt.wantLimit)
			}

			checkInt64Ptr(t, "Usage", info.Usage, tt.wantUsage)

			if tt.wantDrive != nil {
				checkInt64Ptr(t, "UsageInDrive", info.UsageInDrive, *tt.wantDrive)
			}

			if tt.wantTrash != nil {
				checkInt64Ptr(t, "UsageInDriveTrash", info.UsageInDriveTrash, *tt.wantTrash)
			}

			if !reflect.DeepEqual(info.FreeBytes, tt.wantFree) {
				t.Errorf(
					"FreeBytes = %v, want %v",
					info.FreeBytes,
					tt.wantFree,
				)
			}
		})
	}
}

func TestGetStorageInfo_QuotaUnavailable(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{}`)
	})

	info, err := GetStorageInfo(context.Background(), svc)

	if !errors.Is(err, ErrQuotaUnavailable) {
		t.Fatalf("err = %v, want %v", err, ErrQuotaUnavailable)
	}

	if info != nil {
		t.Errorf("info = %+v, want nil", info)
	}
}

func TestGetStorageInfo_APIError(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(
			w,
			http.StatusInternalServerError,
			`{"error":{"code":500,"message":"boom"}}`,
		)
	})

	_, err := GetStorageInfo(context.Background(), svc)

	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "failed to retrieve storage information") {
		t.Errorf("error should be wrapped with context: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetDriveOverview
// ---------------------------------------------------------------------------

func TestGetDriveOverview_Success(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case isFilesRequest(r):
			writeJSON(w, http.StatusOK, `{
				"files": [{
					"id": "file-1",
					"name": "test.txt",
					"size": "10"
				}]
			}`)

		case isAboutRequest(r):
			writeJSON(w, http.StatusOK, `{
				"storageQuota": {
					"limit": "10000",
					"usage": "4000"
				}
			}`)

		default:
			http.NotFound(w, r)
		}
	})

	overview, err := GetDriveOverview(context.Background(), svc)
	if err != nil {
		t.Fatalf("GetDriveOverview: %v", err)
	}

	if len(overview.Files) != 1 ||
		overview.Files[0].ID != "file-1" ||
		overview.Files[0].Size != 10 {
		t.Errorf("Files = %+v", overview.Files)
	}

	checkInt64Ptr(t, "Storage.Limit", overview.Storage.Limit, 10000)
	checkInt64Ptr(t, "Storage.FreeBytes", overview.Storage.FreeBytes, 6000)
}

func TestGetDriveOverview_NilService(t *testing.T) {
	overview, err := GetDriveOverview(context.Background(), nil)

	if !errors.Is(err, ErrInvalidService) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidService)
	}

	if overview != nil {
		t.Errorf("overview = %+v, want nil", overview)
	}
}

func TestGetDriveOverview_ListFilesFails(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if isFilesRequest(r) {
			writeJSON(
				w,
				http.StatusForbidden,
				`{"error":{"code":403,"message":"forbidden"}}`,
			)
			return
		}

		writeJSON(w, http.StatusOK, `{
			"storageQuota": {
				"limit": "100",
				"usage": "40"
			}
		}`)
	})

	overview, err := GetDriveOverview(context.Background(), svc)

	if err == nil {
		t.Fatal("expected an error")
	}

	if overview != nil {
		t.Errorf("overview = %+v, want nil", overview)
	}
}

func TestGetDriveOverview_StorageInfoFails(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if isFilesRequest(r) {
			writeJSON(w, http.StatusOK, `{"files":[]}`)
			return
		}

		writeJSON(w, http.StatusOK, `{}`)
	})

	overview, err := GetDriveOverview(context.Background(), svc)

	if !errors.Is(err, ErrQuotaUnavailable) {
		t.Fatalf("err = %v, want %v", err, ErrQuotaUnavailable)
	}

	if overview != nil {
		t.Errorf("overview = %+v, want nil", overview)
	}
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func TestInt64Pointer(t *testing.T) {
	p := int64Pointer(42)

	if p == nil || *p != 42 {
		t.Fatalf("int64Pointer(42) = %v", p)
	}

	// Each call must return an independent pointer.
	q := int64Pointer(42)

	if p == q {
		t.Error("int64Pointer should return distinct pointers")
	}
}
