package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerConfig 追踪配置
type TracerConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	SampleRate     float64
	Enabled        bool
}

// Tracer 分布式追踪器
type Tracer struct {
	config   *TracerConfig
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// NewTracer 创建追踪器
func NewTracer(config *TracerConfig) (*Tracer, error) {
	if !config.Enabled {
		return &Tracer{
			config: config,
			tracer: otel.Tracer(config.ServiceName),
		}, nil
	}

	ctx := context.Background()

	// 创建 OTLP 导出器
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("创建 OTLP 导出器失败: %w", err)
	}

	// 创建资源
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("创建资源失败: %w", err)
	}

	// 创建采样器
	var sampler sdktrace.Sampler
	if config.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SampleRate)
	}

	// 创建 TracerProvider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(provider)

	// 设置全局传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{
		config:   config,
		provider: provider,
		tracer:   provider.Tracer(config.ServiceName),
	}, nil
}

// Shutdown 关闭追踪器
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// StartSpan 开始一个新的 Span
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// SpanFromContext 从上下文获取当前 Span
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddEvent 添加事件到当前 Span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes 设置属性到当前 Span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError 记录错误到当前 Span
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}

// GetTraceID 获取当前追踪 ID
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID 获取当前 Span ID
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// SpanKind Span 类型
type SpanKind = trace.SpanKind

const (
	SpanKindServer   = trace.SpanKindServer
	SpanKindClient   = trace.SpanKindClient
	SpanKindProducer = trace.SpanKindProducer
	SpanKindConsumer = trace.SpanKindConsumer
	SpanKindInternal = trace.SpanKindInternal
)

// WithSpanKind 设置 Span 类型
func WithSpanKind(kind SpanKind) trace.SpanStartOption {
	return trace.WithSpanKind(kind)
}

// WithAttributes 设置 Span 属性
func WithAttributes(attrs ...attribute.KeyValue) trace.SpanStartOption {
	return trace.WithAttributes(attrs...)
}

// Attribute 辅助函数
func StringAttr(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

func IntAttr(key string, value int) attribute.KeyValue {
	return attribute.Int(key, value)
}

func Int64Attr(key string, value int64) attribute.KeyValue {
	return attribute.Int64(key, value)
}

func BoolAttr(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

func Float64Attr(key string, value float64) attribute.KeyValue {
	return attribute.Float64(key, value)
}

// TraceFunc 追踪函数执行
func TraceFunc(ctx context.Context, tracer *Tracer, name string, fn func(context.Context) error) error {
	ctx, span := tracer.StartSpan(ctx, name)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
	}

	return err
}

// HTTPAttributes HTTP 请求属性
func HTTPAttributes(method, path string, statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.HTTPMethod(method),
		semconv.HTTPRoute(path),
		semconv.HTTPStatusCode(statusCode),
	}
}

// DBAttributes 数据库操作属性
func DBAttributes(operation, table string) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.DBOperation(operation),
		semconv.DBSQLTable(table),
	}
}

// Service 链路追踪服务
type Service struct {
	tracer *Tracer
	config *TracerConfig
}

// NewService 创建链路追踪服务
func NewService(tracer *Tracer, config *TracerConfig) *Service {
	return &Service{
		tracer: tracer,
		config: config,
	}
}

// IsEnabled 检查是否启用
func (s *Service) IsEnabled() bool {
	return s.config != nil && s.config.Enabled
}

// GetTracer 获取追踪器
func (s *Service) GetTracer() *Tracer {
	return s.tracer
}

// GetConfig 获取配置
func (t *Tracer) GetConfig() *TracerConfig {
	return t.config
}

// QueryTraces 查询 Traces
func (s *Service) QueryTraces(ctx context.Context, req *TraceQuery) (*TraceListResponse, error) {
	traces := []TraceRecord{}
	total := int64(0)

	return &TraceListResponse{
		Total:   total,
		Traces:  traces,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}, nil
}

// GetTraceByID 获取单个 Trace
func (s *Service) GetTraceByID(ctx context.Context, traceID string) (*TraceRecord, error) {
	return nil, fmt.Errorf("trace not found")
}

// GetTraceTree 获取 Trace 树形结构
func (s *Service) GetTraceTree(ctx context.Context, traceID string) (*TraceRecord, []TraceRecord, error) {
	return nil, nil, fmt.Errorf("trace not found")
}

// ListServices 获取服务列表
func (s *Service) ListServices(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

// GetServiceOperations 获取服务操作列表
func (s *Service) GetServiceOperations(ctx context.Context, serviceName string) ([]string, error) {
	return []string{}, nil
}

// TraceQuery Trace 查询参数
type TraceQuery struct {
	ServiceName string
	Operation   string
	StartTime   time.Time
	EndTime     time.Time
	Limit       int
	Offset      int
}

// TraceListResponse Trace 列表响应
type TraceListResponse struct {
	Total   int64
	Traces  []TraceRecord
	Limit   int
	Offset  int
}

// TraceRecord Trace 记录
type TraceRecord struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Operation  string                 `json:"operation"`
	Service    string                 `json:"service"`
	Kind       string                 `json:"kind"`
	StartTime  time.Time             `json:"start_time"`
	EndTime    time.Time             `json:"end_time"`
	Duration   int64                  `json:"duration_ms"`
	Status     string                 `json:"status"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
	Attributes map[string]interface{} `json:"attributes"`
	Events     []TraceEvent           `json:"events,omitempty"`
}

// TraceEvent Trace 事件
type TraceEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Name      string                 `json:"name"`
	Attributes map[string]interface{} `json:"attributes"`
}
