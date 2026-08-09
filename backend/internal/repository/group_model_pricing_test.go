//go:build unit

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func bytesPtr(s string) *[]byte {
	b := []byte(s)
	return &b
}

func TestUnmarshalModelPricing(t *testing.T) {
	t.Run("nil 列返回 nil", func(t *testing.T) {
		require.Nil(t, unmarshalModelPricing(nil))
	})

	t.Run("空字节返回 nil", func(t *testing.T) {
		require.Nil(t, unmarshalModelPricing(bytesPtr("")))
	})

	t.Run("空 JSON 对象返回 nil 以走分组全局价", func(t *testing.T) {
		require.Nil(t, unmarshalModelPricing(bytesPtr(`{}`)))
	})

	t.Run("两个 map 都为空时返回 nil", func(t *testing.T) {
		require.Nil(t, unmarshalModelPricing(bytesPtr(`{"video":{},"image":{}}`)))
	})

	t.Run("脏数据静默降级为 nil", func(t *testing.T) {
		require.Nil(t, unmarshalModelPricing(bytesPtr(`not-json`)))
		require.Nil(t, unmarshalModelPricing(bytesPtr(`{"video":"should-be-object"}`)))
	})

	t.Run("视频按秒定价", func(t *testing.T) {
		got := unmarshalModelPricing(bytesPtr(`{"video":{"veo-3.1":{"per_second":0.02}}}`))
		require.NotNil(t, got)
		price := got.GetModelVideoPrice("veo-3.1")
		require.NotNil(t, price)
		require.NotNil(t, price.PricePerSecond)
		require.InDelta(t, 0.02, *price.PricePerSecond, 1e-9)
		require.Nil(t, price.PricePerCount)
	})

	t.Run("视频按次定价", func(t *testing.T) {
		got := unmarshalModelPricing(bytesPtr(`{"video":{"kling-3.0":{"per_count":0.5}}}`))
		require.NotNil(t, got)
		price := got.GetModelVideoPrice("kling-3.0")
		require.NotNil(t, price)
		require.NotNil(t, price.PricePerCount)
		require.InDelta(t, 0.5, *price.PricePerCount, 1e-9)
	})

	t.Run("图片按尺寸档位定价", func(t *testing.T) {
		got := unmarshalModelPricing(bytesPtr(`{"image":{"gpt-image-2":{"1k":0.01,"2k":0.03,"4k":0.09}}}`))
		require.NotNil(t, got)
		p1k := got.GetModelImagePrice("gpt-image-2", "1K")
		require.NotNil(t, p1k)
		require.InDelta(t, 0.01, *p1k, 1e-9)
		p4k := got.GetModelImagePrice("gpt-image-2", "4K")
		require.NotNil(t, p4k)
		require.InDelta(t, 0.09, *p4k, 1e-9)
	})

	t.Run("图片与视频混合配置", func(t *testing.T) {
		raw := `{"video":{"veo-3.1":{"per_second":0.02},"kling-3.0":{"per_count":0.5}},` +
			`"image":{"gpt-image-2":{"2k":0.03},"nano-banana-2":{"1k":0.005}}}`
		got := unmarshalModelPricing(bytesPtr(raw))
		require.NotNil(t, got)
		require.Len(t, got.Video, 2)
		require.Len(t, got.Image, 2)
	})

	t.Run("零价格是合法配置不能被当成未配置", func(t *testing.T) {
		got := unmarshalModelPricing(bytesPtr(`{"video":{"free-model":{"per_count":0}}}`))
		require.NotNil(t, got)
		price := got.GetModelVideoPrice("free-model")
		require.NotNil(t, price)
		require.NotNil(t, price.PricePerCount)
		require.Zero(t, *price.PricePerCount)
	})
}

// 写入侧序列化与读取侧反序列化必须互逆，否则「保存后再打开」会丢配置。
func TestModelPricingRoundTrip(t *testing.T) {
	src := &service.ModelPricingConfig{
		Video: map[string]*service.ModelVideoPricing{
			"veo-3.1":   {PricePerSecond: ptrFloat(0.02)},
			"kling-3.0": {PricePerCount: ptrFloat(0.5)},
		},
		Image: map[string]*service.ModelImagePricing{
			"gpt-image-2": {Price1K: ptrFloat(0.01), Price2K: ptrFloat(0.03), Price4K: ptrFloat(0.09)},
		},
	}

	encoded, err := json.Marshal(src)
	require.NoError(t, err)

	decoded := unmarshalModelPricing(&encoded)
	require.NotNil(t, decoded)
	require.Len(t, decoded.Video, 2)
	require.Len(t, decoded.Image, 1)

	require.InDelta(t, 0.02, *decoded.Video["veo-3.1"].PricePerSecond, 1e-9)
	require.InDelta(t, 0.5, *decoded.Video["kling-3.0"].PricePerCount, 1e-9)
	require.InDelta(t, 0.03, *decoded.Image["gpt-image-2"].Price2K, 1e-9)

	// 未设置的字段往返后仍为 nil（omitempty 不能把 nil 变成 0）
	require.Nil(t, decoded.Video["veo-3.1"].PricePerCount)
	require.Nil(t, decoded.Video["kling-3.0"].PricePerSecond)
}

func ptrFloat(v float64) *float64 { return &v }
