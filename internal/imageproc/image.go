// Package imageproc 提供图像合成与处理功能。
package imageproc

import "fmt"

// Processor 图像处理器
type Processor struct{}

// NewProcessor 创建图像处理器
func NewProcessor() *Processor {
	return &Processor{}
}

// Composite 合成图像（文字 + 图片叠加）
func (p *Processor) Composite(background []byte, text string, opts map[string]any) ([]byte, error) {
	return nil, fmt.Errorf("图像合成尚未实现")
}

// Resize 调整图像大小
func (p *Processor) Resize(input []byte, width, height int) ([]byte, error) {
	return nil, fmt.Errorf("图像缩放尚未实现")
}

// SVGToPNG 将 SVG 渲染为 PNG
func (p *Processor) SVGToPNG(svg []byte, scale float64) ([]byte, error) {
	return nil, fmt.Errorf("SVG 渲染尚未实现")
}

// SupportFormats 返回支持的图像格式
func SupportFormats() []string {
	return []string{"png", "jpg", "gif", "webp"}
}
