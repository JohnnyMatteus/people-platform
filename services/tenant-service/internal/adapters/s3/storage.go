// Package s3 implementa app.AssetStorage com Amazon S3. No ambiente local, o mesmo código
// aponta para o LocalStack trocando apenas o endpoint.
package s3

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
)

// Storage gera URLs pré-assinadas de upload para o bucket de assets dos tenants.
type Storage struct {
	presign       *s3.PresignClient
	bucket        string
	publicBaseURL string
}

var _ app.AssetStorage = (*Storage)(nil)

// New cria o adaptador. endpoint vazio usa o S3 da AWS; com endpoint (LocalStack), usa
// path-style, que é o que emuladores aceitam sem DNS curinga. publicBaseURL é a base por onde
// o navegador lê os objetos (CDN em produção).
func New(cfg aws.Config, bucket, endpoint, publicBaseURL string) *Storage {
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})
	return &Storage{presign: s3.NewPresignClient(client), bucket: bucket, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}

// PresignUpload assina localmente (sem chamada de rede) um PUT com tipo de conteúdo fixo.
func (s *Storage) PresignUpload(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// PublicURL devolve a URL de leitura do objeto.
func (s *Storage) PublicURL(key string) string { return s.publicBaseURL + "/" + key }
