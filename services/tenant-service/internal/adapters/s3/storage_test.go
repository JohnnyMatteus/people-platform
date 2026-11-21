package s3

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func TestPresignUpload_LocalEndpointPathStyle(t *testing.T) {
	cfg := aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")}
	s := New(cfg, "people-platform-tenant-assets", "http://localhost:4566", "http://localhost:4566/people-platform-tenant-assets/")

	raw, err := s.PresignUpload(context.Background(), "tenants/t1/logo/a.png", "image/png", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "localhost:4566" || u.Path != "/people-platform-tenant-assets/tenants/t1/logo/a.png" {
		t.Fatalf("URL pré-assinada inesperada: %s", raw)
	}
	q := u.Query()
	if q.Get("X-Amz-Expires") != "600" || q.Get("X-Amz-Signature") == "" {
		t.Fatalf("assinatura ou expiração ausentes: %s", raw)
	}
	if !strings.Contains(q.Get("X-Amz-SignedHeaders"), "content-type") {
		t.Fatalf("content-type deve fazer parte da assinatura: %s", q.Get("X-Amz-SignedHeaders"))
	}
	if got := s.PublicURL("tenants/t1/logo/a.png"); got != "http://localhost:4566/people-platform-tenant-assets/tenants/t1/logo/a.png" {
		t.Fatalf("PublicURL = %s", got)
	}
}
