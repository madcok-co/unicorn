// Package metrics provides a generic metrics adapter
// that wraps any metrics library (prometheus, datadog, statsd, etc.)
package metrics

import (
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver is the interface that any metrics provider must implement
type Driver interface {
	Counter(name string, tags []contracts.Tag) CounterDriver
	Gauge(name string, tags []contracts.Tag) GaugeDriver
	Histogram(name string, tags []contracts.Tag) HistogramDriver
	Handler() any
	Close() error
}

// CounterDriver is the interface for counters
type CounterDriver interface {
	Inc()
	Add(delta float64)
}

// GaugeDriver is the interface for gauges
type GaugeDriver interface {
	Set(value float64)
	Inc()
	Dec()
	Add(delta float64)
	Sub(delta float64)
}

// HistogramDriver is the interface for histograms
type HistogramDriver interface {
	Observe(value float64)
}

// Adapter implements contracts.Metrics
type Adapter struct {
	driver      Driver
	namespace   string
	defaultTags []contracts.Tag
}

// New creates a new metrics adapter
func New(driver Driver) *Adapter {
	return &Adapter{
		driver:      driver,
		defaultTags: make([]contracts.Tag, 0),
	}
}

// WithNamespace sets metrics namespace/prefix
func (a *Adapter) WithNamespace(ns string) *Adapter {
	a.namespace = ns
	return a
}

// WithDefaultTags sets default tags for all metrics
func (a *Adapter) WithDefaultTags(tags ...contracts.Tag) *Adapter {
	a.defaultTags = tags
	return a
}

// ============ contracts.Metrics Implementation ============

func (a *Adapter) Counter(name string, tags ...contracts.Tag) contracts.Counter {
	fullName := a.prefixName(name)
	allTags := a.mergeTags(tags)
	return &counterWrapper{driver: a.driver.Counter(fullName, allTags)}
}

func (a *Adapter) Gauge(name string, tags ...contracts.Tag) contracts.Gauge {
	fullName := a.prefixName(name)
	allTags := a.mergeTags(tags)
	return &gaugeWrapper{driver: a.driver.Gauge(fullName, allTags)}
}

func (a *Adapter) Histogram(name string, tags ...contracts.Tag) contracts.Histogram {
	fullName := a.prefixName(name)
	allTags := a.mergeTags(tags)
	return &histogramWrapper{driver: a.driver.Histogram(fullName, allTags)}
}

func (a *Adapter) Timer(name string, tags ...contracts.Tag) contracts.Timer {
	return &timer{histogram: a.Histogram(name, tags...)}
}

func (a *Adapter) WithTags(tags ...contracts.Tag) contracts.Metrics {
	return &Adapter{
		driver:      a.driver,
		namespace:   a.namespace,
		defaultTags: a.mergeTags(tags),
	}
}

func (a *Adapter) Handler() any {
	return a.driver.Handler()
}

func (a *Adapter) Close() error {
	return a.driver.Close()
}

func (a *Adapter) prefixName(name string) string {
	if a.namespace == "" {
		return name
	}
	return a.namespace + "_" + name
}

func (a *Adapter) mergeTags(tags []contracts.Tag) []contracts.Tag {
	if len(a.defaultTags) == 0 {
		return tags
	}
	result := make([]contracts.Tag, 0, len(a.defaultTags)+len(tags))
	result = append(result, a.defaultTags...)
	result = append(result, tags...)
	return result
}

// ============ Wrapper Types ============

type counterWrapper struct {
	driver CounterDriver
}

func (c *counterWrapper) Inc() {
	c.driver.Inc()
}

func (c *counterWrapper) Add(delta float64) {
	c.driver.Add(delta)
}

type gaugeWrapper struct {
	driver GaugeDriver
}

func (g *gaugeWrapper) Set(value float64) {
	g.driver.Set(value)
}

func (g *gaugeWrapper) Inc() {
	g.driver.Inc()
}

func (g *gaugeWrapper) Dec() {
	g.driver.Dec()
}

func (g *gaugeWrapper) Add(delta float64) {
	g.driver.Add(delta)
}

func (g *gaugeWrapper) Sub(delta float64) {
	g.driver.Sub(delta)
}

type histogramWrapper struct {
	driver HistogramDriver
}

func (h *histogramWrapper) Observe(value float64) {
	h.driver.Observe(value)
}

type timer struct {
	histogram contracts.Histogram
}

func (t *timer) Start() func() {
	start := time.Now()
	return func() {
		t.histogram.Observe(time.Since(start).Seconds())
	}
}

func (t *timer) Record(duration time.Duration) {
	t.histogram.Observe(duration.Seconds())
}

func (t *timer) Time(fn func()) {
	start := time.Now()
	fn()
	t.histogram.Observe(time.Since(start).Seconds())
}

// ============ In-Memory Metrics Driver ============

// MemoryDriver stores metrics in memory (useful for testing)
type MemoryDriver struct {
	counters   map[string]*memoryCounter
	gauges     map[string]*memoryGauge
	histograms map[string]*memoryHistogram
	mu         sync.RWMutex
}

// NewMemoryDriver creates an in-memory metrics driver
func NewMemoryDriver() *MemoryDriver {
	return &MemoryDriver{
		counters:   make(map[string]*memoryCounter),
		gauges:     make(map[string]*memoryGauge),
		histograms: make(map[string]*memoryHistogram),
	}
}

func (d *MemoryDriver) Counter(name string, tags []contracts.Tag) CounterDriver {
	key := d.makeKey(name, tags)
	d.mu.Lock()
	defer d.mu.Unlock()

	if c, ok := d.counters[key]; ok {
		return c
	}

	c := &memoryCounter{}
	d.counters[key] = c
	return c
}

func (d *MemoryDriver) Gauge(name string, tags []contracts.Tag) GaugeDriver {
	key := d.makeKey(name, tags)
	d.mu.Lock()
	defer d.mu.Unlock()

	if g, ok := d.gauges[key]; ok {
		return g
	}

	g := &memoryGauge{}
	d.gauges[key] = g
	return g
}

func (d *MemoryDriver) Histogram(name string, tags []contracts.Tag) HistogramDriver {
	key := d.makeKey(name, tags)
	d.mu.Lock()
	defer d.mu.Unlock()

	if h, ok := d.histograms[key]; ok {
		return h
	}

	h := &memoryHistogram{}
	d.histograms[key] = h
	return h
}

func (d *MemoryDriver) Handler() any {
	return nil
}

func (d *MemoryDriver) Close() error {
	return nil
}

func (d *MemoryDriver) makeKey(name string, tags []contracts.Tag) string {
	key := name
	for _, tag := range tags {
		key += "|" + tag.Key + "=" + tag.Value
	}
	return key
}

// GetCounter returns counter value for testing
func (d *MemoryDriver) GetCounter(name string, tags ...contracts.Tag) float64 {
	key := d.makeKey(name, tags)
	d.mu.RLock()
	defer d.mu.RUnlock()
	if c, ok := d.counters[key]; ok {
		return c.value
	}
	return 0
}

// GetGauge returns gauge value for testing
func (d *MemoryDriver) GetGauge(name string, tags ...contracts.Tag) float64 {
	key := d.makeKey(name, tags)
	d.mu.RLock()
	defer d.mu.RUnlock()
	if g, ok := d.gauges[key]; ok {
		return g.value
	}
	return 0
}

// GetHistogramCount returns histogram observation count for testing
func (d *MemoryDriver) GetHistogramCount(name string, tags ...contracts.Tag) int {
	key := d.makeKey(name, tags)
	d.mu.RLock()
	defer d.mu.RUnlock()
	if h, ok := d.histograms[key]; ok {
		return len(h.values)
	}
	return 0
}

type memoryCounter struct {
	value float64
	mu    sync.Mutex
}

func (c *memoryCounter) Inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *memoryCounter) Add(delta float64) {
	c.mu.Lock()
	c.value += delta
	c.mu.Unlock()
}

type memoryGauge struct {
	value float64
	mu    sync.Mutex
}

func (g *memoryGauge) Set(value float64) {
	g.mu.Lock()
	g.value = value
	g.mu.Unlock()
}

func (g *memoryGauge) Inc() {
	g.mu.Lock()
	g.value++
	g.mu.Unlock()
}

func (g *memoryGauge) Dec() {
	g.mu.Lock()
	g.value--
	g.mu.Unlock()
}

func (g *memoryGauge) Add(delta float64) {
	g.mu.Lock()
	g.value += delta
	g.mu.Unlock()
}

func (g *memoryGauge) Sub(delta float64) {
	g.mu.Lock()
	g.value -= delta
	g.mu.Unlock()
}

type memoryHistogram struct {
	values []float64
	mu     sync.Mutex
}

func (h *memoryHistogram) Observe(value float64) {
	h.mu.Lock()
	h.values = append(h.values, value)
	h.mu.Unlock()
}

// ============ Noop Driver ============

// NoopDriver discards all metrics
type NoopDriver struct{}

// NewNoopDriver creates a no-op driver
func NewNoopDriver() *NoopDriver {
	return &NoopDriver{}
}

func (d *NoopDriver) Counter(name string, tags []contracts.Tag) CounterDriver {
	return &noopCounter{}
}

func (d *NoopDriver) Gauge(name string, tags []contracts.Tag) GaugeDriver {
	return &noopGauge{}
}

func (d *NoopDriver) Histogram(name string, tags []contracts.Tag) HistogramDriver {
	return &noopHistogram{}
}

func (d *NoopDriver) Handler() any { return nil }
func (d *NoopDriver) Close() error { return nil }

type noopCounter struct{}

func (c *noopCounter) Inc()        {}
func (c *noopCounter) Add(float64) {}

type noopGauge struct{}

func (g *noopGauge) Set(float64) {}
func (g *noopGauge) Inc()        {}
func (g *noopGauge) Dec()        {}
func (g *noopGauge) Add(float64) {}
func (g *noopGauge) Sub(float64) {}

type noopHistogram struct{}

func (h *noopHistogram) Observe(float64) {}

// ============ Prometheus Wrapper ============

// PrometheusRegistry is the interface that prometheus registries implement
type PrometheusRegistry interface {
	MustRegister(collectors ...PrometheusCollector)
}

// PrometheusCollector is a prometheus collector
type PrometheusCollector interface{}

// PrometheusCounter is the interface for prometheus counters
type PrometheusCounter interface {
	Inc()
	Add(float64)
	WithLabelValues(lvs ...string) PrometheusCounter
}

// PrometheusGauge is the interface for prometheus gauges
type PrometheusGauge interface {
	Set(float64)
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
	WithLabelValues(lvs ...string) PrometheusGauge
}

// PrometheusHistogram is the interface for prometheus histograms
type PrometheusHistogram interface {
	Observe(float64)
	WithLabelValues(lvs ...string) PrometheusHistogram
}

// PrometheusDriver wraps prometheus client
type PrometheusDriver struct {
	registry   PrometheusRegistry
	counters   map[string]PrometheusCounter
	gauges     map[string]PrometheusGauge
	histograms map[string]PrometheusHistogram
	handler    any
	mu         sync.RWMutex
}

// NewPrometheusDriver creates a prometheus driver
// Usage:
//
//	import "github.com/prometheus/client_golang/prometheus"
//	import "github.com/prometheus/client_golang/prometheus/promhttp"
//
//	driver := metrics.NewPrometheusDriver(prometheus.DefaultRegisterer, promhttp.Handler())
//	adapter := metrics.New(driver)
func NewPrometheusDriver(registry PrometheusRegistry, handler any) *PrometheusDriver {
	return &PrometheusDriver{
		registry:   registry,
		counters:   make(map[string]PrometheusCounter),
		gauges:     make(map[string]PrometheusGauge),
		histograms: make(map[string]PrometheusHistogram),
		handler:    handler,
	}
}

// RegisterCounter registers a prometheus counter
func (d *PrometheusDriver) RegisterCounter(name string, counter PrometheusCounter) {
	d.mu.Lock()
	d.counters[name] = counter
	d.mu.Unlock()
}

// RegisterGauge registers a prometheus gauge
func (d *PrometheusDriver) RegisterGauge(name string, gauge PrometheusGauge) {
	d.mu.Lock()
	d.gauges[name] = gauge
	d.mu.Unlock()
}

// RegisterHistogram registers a prometheus histogram
func (d *PrometheusDriver) RegisterHistogram(name string, histogram PrometheusHistogram) {
	d.mu.Lock()
	d.histograms[name] = histogram
	d.mu.Unlock()
}

func (d *PrometheusDriver) Counter(name string, tags []contracts.Tag) CounterDriver {
	d.mu.RLock()
	c, ok := d.counters[name]
	d.mu.RUnlock()

	if !ok {
		return &noopCounter{}
	}

	if len(tags) > 0 {
		labels := make([]string, len(tags))
		for i, tag := range tags {
			labels[i] = tag.Value
		}
		return &promCounterWrapper{counter: c.WithLabelValues(labels...)}
	}

	return &promCounterWrapper{counter: c}
}

func (d *PrometheusDriver) Gauge(name string, tags []contracts.Tag) GaugeDriver {
	d.mu.RLock()
	g, ok := d.gauges[name]
	d.mu.RUnlock()

	if !ok {
		return &noopGauge{}
	}

	if len(tags) > 0 {
		labels := make([]string, len(tags))
		for i, tag := range tags {
			labels[i] = tag.Value
		}
		return &promGaugeWrapper{gauge: g.WithLabelValues(labels...)}
	}

	return &promGaugeWrapper{gauge: g}
}

func (d *PrometheusDriver) Histogram(name string, tags []contracts.Tag) HistogramDriver {
	d.mu.RLock()
	h, ok := d.histograms[name]
	d.mu.RUnlock()

	if !ok {
		return &noopHistogram{}
	}

	if len(tags) > 0 {
		labels := make([]string, len(tags))
		for i, tag := range tags {
			labels[i] = tag.Value
		}
		return &promHistogramWrapper{histogram: h.WithLabelValues(labels...)}
	}

	return &promHistogramWrapper{histogram: h}
}

func (d *PrometheusDriver) Handler() any {
	return d.handler
}

func (d *PrometheusDriver) Close() error {
	return nil
}

type promCounterWrapper struct {
	counter PrometheusCounter
}

func (c *promCounterWrapper) Inc() {
	c.counter.Inc()
}

func (c *promCounterWrapper) Add(delta float64) {
	c.counter.Add(delta)
}

type promGaugeWrapper struct {
	gauge PrometheusGauge
}

func (g *promGaugeWrapper) Set(value float64) {
	g.gauge.Set(value)
}

func (g *promGaugeWrapper) Inc() {
	g.gauge.Inc()
}

func (g *promGaugeWrapper) Dec() {
	g.gauge.Dec()
}

func (g *promGaugeWrapper) Add(delta float64) {
	g.gauge.Add(delta)
}

func (g *promGaugeWrapper) Sub(delta float64) {
	g.gauge.Sub(delta)
}

type promHistogramWrapper struct {
	histogram PrometheusHistogram
}

func (h *promHistogramWrapper) Observe(value float64) {
	h.histogram.Observe(value)
}
