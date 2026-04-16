package gominio

import (
	"context"
	"testing"

	minio "github.com/minio/minio-go/v7"
)

func TestNewClient(t *testing.T) {
	t.Run("缺少 access key 报错", func(t *testing.T) {
		_, err := NewClient(Config{
			Endpoint:        "127.0.0.1:9000",
			SecretAccessKey: "12345678",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("缺少 secret key 报错", func(t *testing.T) {
		_, err := NewClient(Config{
			Endpoint:    "127.0.0.1:9000",
			AccessKeyID: "admin",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("合法配置可初始化", func(t *testing.T) {
		client, err := NewClient(Config{
			Endpoint:        "127.0.0.1:9000",
			AccessKeyID:     "admin",
			SecretAccessKey: "12345678",
			UseSSL:          false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatalf("expected client, got nil")
		}
		if client.SDK() == nil {
			t.Fatalf("expected sdk client, got nil")
		}
	})
}

func TestUploadFile(t *testing.T) {
	t.Run("上传成功", func(t *testing.T) {
		client, err := NewClient(Config{
			Endpoint:        "127.0.0.1:9000",
			AccessKeyID:     "admin",
			SecretAccessKey: "12345678",
			UseSSL:          false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx := context.Background()
		info, err := client.UploadFile(ctx, "test", "images", "C:\\Users\\v_yudlin\\Pictures\\mermaid_20260325170932.png", minio.PutObjectOptions{})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 打印info
		t.Logf("info: %v", info)
	})
}

func TestUpGetObject(t *testing.T) {
	t.Run("", func(t *testing.T) {
		client, err := NewClient(Config{
			Endpoint:        "127.0.0.1:9000",
			AccessKeyID:     "admin",
			SecretAccessKey: "12345678",
			UseSSL:          false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx := context.Background()
		obj, err := client.GetObject(ctx, "test", "images/mermaid_20260325170932.png")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		info, err := obj.Stat()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Logf("info: %v", info)
		obj.Close()
	})
}
