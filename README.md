# go_minio

基于 Golang 的 MinIO 工具包。

当前对外提供：
- 初始化 MinIO 客户端 API
- 上传本地文件 API
- 上传 `io.Reader` 资源 API
- 获取对象资源 API
- 获取obj_url API



## 安装

官方 SDK 安装命令：

```bash
go get github.com/minio/minio-go/v7
```

说明：当前安装到的最新版本 `github.com/minio/minio-go/v7 v7.0.100` 会将模块 Go 版本提升到 `1.25`。



## 快速使用

```go
package main

import (
	"context"
	"log"

	gominio "go_minio"
	minio "github.com/minio/minio-go/v7"
)


func main() {
	client, err := gominio.NewClient(gominio.Config{
		Endpoint:        "127.0.0.1:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		UseSSL:          false,
	})
	if err != nil {
		log.Fatal(err)
	}

info, err := client.UploadFile(
	context.Background(),
	"demo-bucket",
	"images",
	"./avatar.png",
	minio.PutObjectOptions{},
)



	if err != nil {
		log.Fatal(err)
	}

	log.Printf("upload success: %+v", info)
}
```


# 相关工具类介绍：


## minio.PutObjectOptions

多数场景直接传 `minio.PutObjectOptions{}` 即可；只有在需要元数据、标签、缓存策略、下载头、加密、对象锁或分片调优时再设置字段。

- **常用字段**
  - `UserMetadata`：自定义元数据；普通 key 会转成 `x-amz-meta-*`
  - `UserTags`：对象标签
  - `Progress`：进度 hook，可用于进度条统计
  - `ContentType`：内容类型；为空时默认 `application/octet-stream`
  - `ContentEncoding`：内容编码，如 `gzip`
  - `ContentDisposition`：下载展示方式，如内联/附件、下载文件名
  - `ContentLanguage`：内容语言
  - `CacheControl`：缓存策略
  - `Expires`：过期时间

- **治理与安全**
  - `Mode`：对象锁保留模式
  - `RetainUntilDate`：对象锁保留到期时间
  - `LegalHold`：法律保留状态
  - `ServerSideEncryption`：服务端加密配置
  - `StorageClass`：存储类型
  - `WebsiteRedirectLocation`：静态网站重定向地址

- **上传行为**
  - `PartSize`：分片大小；影响大文件/未知长度流的分片策略与内存占用
  - `NumThreads`：分片上传并发数
  - `DisableMultipart`：禁用分片上传；开启后必须提供已知 `Size`
  - `ConcurrentStreamParts`：对未知长度流启用并行分片上传
  - `SendContentMd5`：发送 `Content-MD5`
  - `DisableContentSha256`：禁用内容 `SHA256`
  - `AutoChecksum`：自动附加校验和；支持时默认 `CRC32C`
  - `Checksum`：强制指定校验和；要求 client 启用 `TrailingHeaders`

- **不建议直接使用**
  - `Internal`：MinIO 内部高级选项，普通业务代码不要使用
  - `customHeaders`：私有字段，外部不能直接赋值；条件上传可用 `SetMatchETag` / `SetMatchETagExcept`

## minio.UploadInfo

`UploadFile` 和 `UploadReader` 成功后都会返回 `minio.UploadInfo`。

- **基础字段**
  - `Bucket`：桶名
  - `Key`：对象完整路径
  - `ETag`：对象 ETag；单次上传时常可作为内容标识，分片上传时不一定等同于文件 MD5
  - `Size`：对象大小，单位字节
  - `LastModified`：最后修改时间
  - `Location`：对象位置信息
  - `VersionID`：对象版本号；只有开启版本控制时才有意义

- **生命周期**
  - `Expiration`：生命周期过期时间
  - `ExpirationRuleID`：命中的生命周期规则 ID
  - 这里不是 HTTP 的 `Expires` 响应头，而是对象生命周期信息

- **校验和**
  - `ChecksumCRC32` / `ChecksumCRC32C` / `ChecksumSHA1` / `ChecksumSHA256` / `ChecksumCRC64NVME`：对象校验和
  - `ChecksumMode`：校验和模式
  - 这些值存在时通常是 **base64 编码**；分片对象时可能是“分片校验和再聚合后的结果”

## minio.Object

`GetObject` 返回的是 `*minio.Object`。它不是简单的 HTTP Body，而是一个支持 `Read` / `ReadAt` / `Seek` / `Stat` / `Close` 的对象读取句柄。

- **调用方真正需要关注的点**
  - 用完后要主动 `Close()`，否则底层请求和资源不会及时释放
  - 如果要对象元信息，调用 `Stat()` 获取
  - 如果只顺序读取，把它当 `io.Reader` 用即可
  - 如果要随机读或断点读，可用 `ReadAt()` / `Seek()`

- **并发与调度**
  - `mutex`：保护对象内部状态，避免多次读写/seek 时状态错乱
  - `reqCh` / `resCh`：和底层 goroutine 通信的请求/响应通道；真正的数据拉取由内部协程完成
  - `ctx` / `cancel`：控制整个对象读取生命周期；`Close()` 时会触发取消

- **读取位置与对象信息**
  - `currOffset`：当前读取偏移量
  - `objectInfo`：缓存的对象元信息，对应 `Stat()` 返回内容
  - `seekData`：标记下次读取前是否需要按新 offset 重新发起拉流
  - `beenRead`：是否已经实际读过数据
  - `objectInfoSet`：是否已经拿到并缓存过对象信息

- **生命周期与错误状态**
  - `isClosed`：是否已经关闭
  - `isStarted`：是否已经发起过第一次操作；首次 `Read` / `Stat` / `Seek` 时会决定内部初始化路径
  - `prevErr`：上一次操作产生的错误；后续调用可能直接复用这个错误返回

- **一句话理解**
  - `minio.Object` 本质上是 **带状态的远程对象读取器**：既保存当前位置，也管理底层 HTTP 流、对象信息缓存、seek/read 行为和关闭状态。









