#!/usr/bin/env bash
# Bootstrap do LocalStack (executado em /etc/localstack/init/ready.d quando o LocalStack fica pronto).
# Cria só o que o domínio usa: buckets de assets white label e de currículos do ATS, e as
# identidades SES dos remetentes dos tenants fictícios. Idempotente.
set -euo pipefail

REGION="${AWS_DEFAULT_REGION:-us-east-1}"
ASSETS_BUCKET="${S3_ASSETS_BUCKET:-people-platform-tenant-assets}"
RESUMES_BUCKET="${S3_RESUMES_BUCKET:-people-platform-recruitment-resumes}"

ensure_bucket() {
  local bucket="$1"
  if ! awslocal s3api head-bucket --bucket "$bucket" 2>/dev/null; then
    awslocal s3api create-bucket --bucket "$bucket" --region "$REGION" >/dev/null
    echo "bucket criado: $bucket"
  fi
}

# Assets white label (logos): upload direto do navegador por URL pré-assinada, leitura pública.
ensure_bucket "$ASSETS_BUCKET"
awslocal s3api put-bucket-cors --bucket "$ASSETS_BUCKET" --cors-configuration '{
  "CORSRules": [{
    "AllowedOrigins": ["http://localhost:5173", "http://*.people.localhost:5173"],
    "AllowedMethods": ["PUT", "GET"],
    "AllowedHeaders": ["Content-Type"],
    "MaxAgeSeconds": 3600
  }]
}'
awslocal s3api put-bucket-policy --bucket "$ASSETS_BUCKET" --policy "{
  \"Version\": \"2012-10-17\",
  \"Statement\": [{\"Sid\": \"PublicReadLogos\", \"Effect\": \"Allow\", \"Principal\": \"*\",
    \"Action\": \"s3:GetObject\", \"Resource\": \"arn:aws:s3:::${ASSETS_BUCKET}/tenants/*/logo/*\"}]
}"

# Currículos do ATS: privado, criptografado e com expiração (minimização de dados, LGPD).
ensure_bucket "$RESUMES_BUCKET"
awslocal s3api put-public-access-block --bucket "$RESUMES_BUCKET" --public-access-block-configuration \
  BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
awslocal s3api put-bucket-encryption --bucket "$RESUMES_BUCKET" --server-side-encryption-configuration \
  '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
awslocal s3api put-bucket-lifecycle-configuration --bucket "$RESUMES_BUCKET" --lifecycle-configuration '{
  "Rules": [{"ID": "expira-curriculos-nao-contratados", "Status": "Enabled",
    "Filter": {"Prefix": "tenants/"}, "Expiration": {"Days": 365}}]
}'

# SES: remetentes white label dos tenants fictícios e da plataforma. O LocalStack entrega no Mailpit (SMTP_HOST).
for sender in no-reply@people.example.com rh@horizonte.example.com pessoas@sol.example.com; do
  awslocal ses verify-email-identity --email-address "$sender" --region "$REGION"
done

echo "bootstrap AWS local concluído"
