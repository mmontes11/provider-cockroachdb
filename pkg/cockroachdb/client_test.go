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
			name:           "contains Authorization header when providing access token",
			clientOpts:     []ClientOption{WithAccessToken("token")},
			reqMethod:      http.MethodGet,
			resStatusCode:  http.StatusOK,
			wantStatusCode: http.StatusOK,
			wantHeaders: map[string]string{
				"Authorization": "Bearer: token",
			},
			wantErr: nil,
		},
		{
			name:          "returns no error for GET 200",
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
		{
			name:      "returns no error for POST 201",
			reqMethod: http.MethodPost,
			reqBody: &testResponse{
				Items: []string{
					"foo",
					"bar",
				},
			},
			resStatusCode:  http.StatusCreated,
			wantStatusCode: http.StatusCreated,
			wantErr:        nil,
		},
		{
			name:           "returns no error for DELETE 204",
			reqMethod:      http.MethodDelete,
			resStatusCode:  http.StatusNoContent,
			wantStatusCode: http.StatusNoContent,
			wantErr:        nil,
		},
		{
			name:          "returns an error for GET 400",
			reqMethod:     http.MethodGet,
			resStatusCode: http.StatusBadRequest,
			resBody: &errorResponse{
				Code:    1,
				Message: "mandatory param: clientId",
			},
			wantStatusCode: http.StatusBadRequest,
			wantErr: &Error{
				ErrorCode: 1,
				HTTPCode:  http.StatusBadRequest,
				Message:   "mandatory param: clientId",
			},
		},
		{
			name:          "returns an error for GET 404",
			reqMethod:     http.MethodGet,
			resStatusCode: http.StatusNotFound,
			resBody: &errorResponse{
				Code:    1,
				Message: "not found",
			},
			wantStatusCode: http.StatusNotFound,
			wantErr: &Error{
				ErrorCode: 1,
				HTTPCode:  http.StatusNotFound,
				Message:   "not found",
			},
		},
		{
			name:          "returns an error for GET 500",
			reqMethod:     http.MethodGet,
			resStatusCode: http.StatusInternalServerError,
			resBody: &errorResponse{
				Code:    1,
				Message: "internal error",
			},
			wantStatusCode: http.StatusInternalServerError,
			wantErr: &Error{
				ErrorCode: 1,
				HTTPCode:  http.StatusInternalServerError,
				Message:   "internal error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.resStatusCode)

				if tt.wantHeaders != nil {
					for k, v := range tt.wantHeaders {
						assert.Equal(t, r.Header.Get(k), v)
					}
				}

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

			res, err := client.do(context.Background(), req, tt.actualBody)

			assert.NotNil(t, res)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, res.StatusCode, tt.wantStatusCode)
			assert.Equal(t, tt.actualBody, tt.wantBody)
		})
	}
}
