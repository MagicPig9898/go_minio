package gominio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config 用于初始化 MinIO 客户端。
// 简单总结：自建 MinIO 场景下，SessionToken 和 Region 通常都不用填，UseSSL 看你服务是否配了 TLS 证书。
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	// STS 临时凭证的会话令牌。使用 长期密钥（固定 AK/SK）时留空即可；如果通过 临时安全凭证（如 AWS STS / MinIO AssumeRole）获取的 AK/SK，就必须把随之下发的 Token 填上，否则鉴权不通过。
	SessionToken string
	// 是否启用 HTTPS。true = HTTPS，false = HTTP。注意：如果 Endpoint 已带 http:// 或 https:// 前缀，代码会从 scheme 自动推断，此字段会被忽略（见 normalizeEndpoint 逻辑）。
	UseSSL bool
	// 对象存储的区域（如 us-east-1、ap-guangzhou）。MinIO 单机/私有化部署通常 留空 就行；对接 AWS S3 或需要签名 V4 区域匹配时才需要填。
	Region string
}

// Client 封装 MinIO 官方客户端，便于对外暴露统一 API。
type Client struct {
	client *minio.Client
}

// GetObjectRequest 定义对象读取参数。
type GetObjectRequest struct {
	BucketName   string
	ObjectPrefix string // 对象前缀，如 images/avatar；可为空。
	FileName     string // 文件名，不应包含路径分隔符。
}

// NewClient 初始化 MinIO 客户端。
func NewClient(cfg Config) (*Client, error) {
	endpoint, secure, err := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(cfg.AccessKeyID) == "" {
		return nil, errors.New("access key id 不能为空")
	}
	if strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, errors.New("secret access key 不能为空")
	}

	rawClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		Secure: secure,
		Region: strings.TrimSpace(cfg.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 minio 客户端失败: %w", err)
	}

	return &Client{client: rawClient}, nil
}

// SDK 返回底层官方 MinIO 客户端，方便调用方扩展使用。
func (c *Client) SDK() *minio.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// UploadFile 上传本地文件。
func (c *Client) UploadFile(ctx context.Context, bucket, prefix, localfilePath string,
	putObjectOptions minio.PutObjectOptions) (minio.UploadInfo, error) {
	if err := c.validate(); err != nil {
		return minio.UploadInfo{}, err
	}

	bucketName := strings.TrimSpace(bucket)
	if bucketName == "" {
		return minio.UploadInfo{}, errors.New("bucket name 不能为空")
	}

	filePath := strings.TrimSpace(localfilePath)
	if filePath == "" {
		return minio.UploadInfo{}, errors.New("file path 不能为空")
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("读取上传文件失败: %w", err)
	}
	if fileInfo.IsDir() {
		return minio.UploadInfo{}, fmt.Errorf("上传文件不能是目录: %s", filePath)
	}

	fileName := filepath.Base(filePath)

	objectName, err := buildObjectName(prefix, fileName)
	if err != nil {
		return minio.UploadInfo{}, err
	}

	uploadInfo, err := c.client.FPutObject(ctx, bucketName, objectName, filePath, putObjectOptions)
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("上传文件失败: %w", err)
	}

	return uploadInfo, nil
}

// UploadReader 以 io.Reader 方式上传内容。
func (c *Client) UploadReader(ctx context.Context, bucket, prefix, fileName string,
	reader io.Reader, size int64, putObjectOptions minio.PutObjectOptions) (minio.UploadInfo, error) {
	if err := c.validate(); err != nil {
		return minio.UploadInfo{}, err
	}

	bucketName := strings.TrimSpace(bucket)
	if bucketName == "" {
		return minio.UploadInfo{}, errors.New("bucket name 不能为空")
	}
	objectName, err := buildObjectName(prefix, fileName)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	if reader == nil {
		return minio.UploadInfo{}, errors.New("reader 不能为空")
	}
	// size 推荐提供，-1 表示未知，0 表示空对象
	if size < -1 {
		return minio.UploadInfo{}, errors.New("size 不能小于 -1")
	}

	uploadInfo, err := c.client.PutObject(ctx, bucketName, objectName, reader, size, putObjectOptions)
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("上传流内容失败: %w", err)
	}

	return uploadInfo, nil
}

// GetObject 获取对象资源流，调用方使用完后应及时关闭 Object；如需对象信息可自行调用 Stat。
func (c *Client) GetObject(ctx context.Context, bucket, key string) (*minio.Object, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	bucketName := strings.TrimSpace(bucket)
	if bucketName == "" {
		return nil, errors.New("bucket name 不能为空")
	}

	object, err := c.client.GetObject(ctx, bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取对象失败: %w", err)
	}

	return object, nil
}

func buildObjectName(prefix string, fileName string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.ReplaceAll(prefix, "\\", "/")
	prefix = strings.Trim(prefix, "/")

	fileName = strings.TrimSpace(fileName)
	fileName = strings.ReplaceAll(fileName, "\\", "/")
	fileName = strings.Trim(fileName, "/")
	if fileName == "" {
		return "", errors.New("file name 不能为空")
	}
	if strings.Contains(fileName, "/") {
		return "", errors.New("file name 不能包含路径分隔符，请将目录放到 object prefix")
	}

	if prefix == "" {
		return fileName, nil
	}
	return prefix + "/" + fileName, nil
}

func (c *Client) validate() error {
	if c == nil || c.client == nil {
		return errors.New("minio 客户端未初始化")
	}
	return nil
}

func normalizeEndpoint(endpoint string, useSSL bool) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, errors.New("endpoint 不能为空")
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", false, fmt.Errorf("解析 endpoint 失败: %w", err)
		}
		if parsed.Host == "" {
			return "", false, errors.New("endpoint 非法")
		}
		return parsed.Host, parsed.Scheme == "https", nil
	}

	return endpoint, useSSL, nil
}
