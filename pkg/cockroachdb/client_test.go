package cockroachdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testResponse struct {
	Items []string `json:"items"`
}

func TestClient(t *testing.T) {
	tests := []struct {
		name       string
		clientOpts []ClientOption

		reqMethod string
		reqBody   interface{}

		resStatusCode int
		resBody       interface{}

		wantStatusCode int
		actualBody     interface{}
		wantBody       interface{}
		wantHeaders    map[string]string
		wantErr        error
	}{
		{
			name:          "returns no error for 200",
			reqMethod:     http.MethodGet,
			resStatusCode: http.StatusOK,
			resBody: &testResponse{
				Items: []string{
					"foo",
					"bar",
				},
			},
			wantStatusCode: http.StatusOK,
			actualBody:     &testResponse{},
			wantBody: &testResponse{
				Items: []string{
					"foo",
					"bar",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.resStatusCode)

				if tt.resBody == nil {
					return
				}
				bytes, err := json.Marshal(tt.resBody)
				if err != nil {
					t.Fatal(err)
				}

				if _, err = w.Write(bytes); err != nil {
					t.Fatal(err)
				}
			}))
			t.Cleanup(ts.Close)

			opts := append(tt.clientOpts, WithBaseURL(ts.URL))
			client, err := NewClient(opts...)
			if err != nil {
				t.Fatal(err)
			}
			assert.NotNil(t, client)

			req, err := client.newRequest(tt.reqMethod, "/api", tt.reqBody)
			if err != nil {
				t.Fatal(err)
			}
			assert.NotNil(t, req)

			for k, v := range tt.wantHeaders {
				assert.Equal(t, req.Header.Get(k), v)
			}

			res, err := client.do(context.Background(), req, tt.actualBody)

			assert.NotNil(t, res)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, res.StatusCode, tt.wantStatusCode)
			assert.Equal(t, tt.actualBody, tt.wantBody)
		})
	}
}
