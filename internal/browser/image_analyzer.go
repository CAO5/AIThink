package browser

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"aithink/internal/config"
)

// ImageAnalyzer 图片分析器
type ImageAnalyzer struct {
	cfgMgr      *config.ConfigManager
	client       *http.Client
	pageQueryFunc func(script string) (interface{}, error) // 用于查询页面元素的回调函数
}

// NewImageAnalyzer 创建图片分析器
func NewImageAnalyzer() *ImageAnalyzer {
	return &ImageAnalyzer{
		cfgMgr: config.GetConfigManager(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetPageQueryFunc 设置页面查询回调函数（用于百度OCR等需要通过JS查询页面的场景）
func (ia *ImageAnalyzer) SetPageQueryFunc(queryFunc func(script string) (interface{}, error)) {
	ia.pageQueryFunc = queryFunc
}

// AnalyzeResult 图片分析结果
type AnalyzeResult struct {
	Success     bool          `json:"success"`
	Message     string        `json:"message"`
	Elements    []PageElement `json:"elements"`
	RawResponse string        `json:"raw_response,omitempty"`
}

// PageElement 页面元素信息
type PageElement struct {
	Type        string `json:"type"`         // button, input, link等
	Text        string `json:"text"`         // 元素文本
	Description string `json:"description"`  // 元素描述
	Selector    string `json:"selector"`     // 建议的选择器
	X           int    `json:"x"`            // X坐标（如果支持）
	Y           int    `json:"y"`            // Y坐标
	Width       int    `json:"width"`        // 宽度
	Height      int    `json:"height"`       // 高度
}

// AnalyzePage 分析页面截图
func (ia *ImageAnalyzer) AnalyzePage(screenshotPath, prompt string) (*AnalyzeResult, error) {
	cfg := ia.cfgMgr.GetConfig()

	if cfg.ImageAI.Provider == "" {
		return nil, fmt.Errorf("未配置图片识别AI服务")
	}

	// 读取截图文件并转换为base64
	imageData, err := readImageBase64(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("读取截图失败: %v", err)
	}

	// 根据配置的provider调用不同的服务
	switch strings.ToLower(cfg.ImageAI.Provider) {
	case "openai":
		return ia.analyzeWithOpenAI(imageData, prompt, cfg.ImageAI.OpenAI)
	case "baidu":
		return ia.analyzeWithBaidu(imageData, prompt, cfg.ImageAI.Baidu)
	case "tencent":
		return ia.analyzeWithTencent(imageData, prompt, cfg.ImageAI.Tencent)
	case "custom":
		return ia.analyzeWithCustomAPI(imageData, prompt, cfg.ImageAI.Custom)
	default:
		return nil, fmt.Errorf("不支持的图片识别AI服务: %s", cfg.ImageAI.Provider)
	}
}

// analyzeWithOpenAI 使用OpenAI GPT-4V分析图片
func (ia *ImageAnalyzer) analyzeWithOpenAI(imageBase64, prompt string, cfg config.OpenAIConfig) (*AnalyzeResult, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API Key未配置")
	}

	// 构建请求
	reqBody := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf(`请分析这张网页截图。%s

请以JSON数组格式返回页面上相关元素的信息，格式如下：
[
  {
    "type": "元素类型(button/input/link等)",
    "text": "元素文本",
    "description": "元素描述",
    "selector": "建议的CSS选择器（如果能确定）",
    "x": 元素中心X坐标（像素）,
    "y": 元素中心Y坐标（像素）,
    "width": 元素宽度,
    "height": 元素高度
  }
]

重要：
1. 必须返回每个元素的 x, y 坐标（图片中的像素坐标）
2. 如果能确定CSS选择器也请一并返回
3. 只返回JSON数组，不要其他内容。`, prompt),
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": fmt.Sprintf("data:image/png;base64,%s", imageBase64),
						},
					},
				},
			},
		},
		"max_tokens": 2000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// 发送请求
	url := cfg.BaseURL + "/chat/completions"
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := ia.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI API返回错误: %s, %s", resp.Status, string(body))
	}

	// 解析响应
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// 提取内容
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					// 解析JSON数组
					var elements []PageElement
					if err := json.Unmarshal([]byte(content), &elements); err != nil {
						// 尝试从文本中提取JSON
						if idx := strings.Index(content, "["); idx >= 0 {
							if endIdx := strings.LastIndex(content, "]"); endIdx > idx {
								jsonStr := content[idx : endIdx+1]
								json.Unmarshal([]byte(jsonStr), &elements)
							}
						}
					}

					return &AnalyzeResult{
						Success:     true,
						Message:     "分析成功",
						Elements:    elements,
						RawResponse: content,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("解析OpenAI响应失败")
}

// analyzeWithBaidu 使用百度OCR分析图片
// 注意：百度OCR只能识别文字，无法直接返回CSS选择器。
// 本实现通过OCR识别文字后，结合页面查询回调函数来定位元素。
func (ia *ImageAnalyzer) analyzeWithBaidu(imageBase64, prompt string, cfg config.BaiduOCRConfig) (*AnalyzeResult, error) {
	// 百度OCR需要先获取access_token
	tokenURL := fmt.Sprintf("https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s",
		cfg.APIKey, cfg.SecretKey)

	resp, err := ia.client.Get(tokenURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResult map[string]interface{}
	json.Unmarshal(body, &tokenResult)

	accessToken, ok := tokenResult["access_token"].(string)
	if !ok {
		return nil, fmt.Errorf("获取百度access_token失败")
	}

	// 调用通用文字识别（含位置信息）
	ocrURL := "https://aip.baidubce.com/rest/2.0/ocr/v1/general_basic?access_token=" + accessToken

	// 去除base64的data URL前缀
	imageBase64 = strings.TrimPrefix(imageBase64, "data:image/png;base64,")

	reqData := "image=" + imageBase64
	req, _ := http.NewRequest("POST", ocrURL, strings.NewReader(reqData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp2, err := ia.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)

	// 解析OCR结果
	var ocrResult map[string]interface{}
	json.Unmarshal(body2, &ocrResult)

	elements := []PageElement{}
	if wordsResult, ok := ocrResult["words_result"].([]interface{}); ok {
		for _, item := range wordsResult {
			if wordInfo, ok := item.(map[string]interface{}); ok {
				if words, ok := wordInfo["words"].(string); ok {
					elements = append(elements, PageElement{
						Type:        "text",
						Text:        words,
						Description: "OCR识别的文本",
					})
				}
			}
		}
	}

	log.Printf("百度OCR识别到%d个文本块", len(elements))

	// 如果配置了页面查询回调，尝试通过OCR识别的文字来定位页面元素
	if ia.pageQueryFunc != nil && len(elements) > 0 {
		log.Println("尝试通过OCR文字结果定位页面元素...")
		elements = ia.findElementsByOCRText(elements)
	}

	return &AnalyzeResult{
		Success: true,
		Message: "OCR识别成功，但无法保证精确返回CSS选择器，建议配置OpenAI等支持视觉理解的AI服务",
		Elements: elements,
	}, nil
}

// findElementsByOCRText 通过OCR识别的文字在页面中查找对应元素
func (ia *ImageAnalyzer) findElementsByOCRText(ocrElements []PageElement) []PageElement {
	result := []PageElement{}

	for _, elem := range ocrElements {
		// 使用JavaScript在页面中查找包含该文字的元素
		script := fmt.Sprintf(`
			() => {
				const searchText = %q;
				const elements = document.querySelectorAll('button, input, a, [role="button"]');
				for (let el of elements) {
					const text = (el.textContent || '').trim();
					const placeholder = (el.placeholder || '').trim();
					const ariaLabel = (el.getAttribute('aria-label') || '').trim();

					if (text.includes(searchText) || placeholder.includes(searchText) || ariaLabel.includes(searchText)) {
						const info = {
							type: el.tagName.toLowerCase(),
							text: text || placeholder || ariaLabel,
							selector: ''
						};
						if (el.id) info.selector = '#' + el.id;
						else if (el.name) info.selector = el.tagName.toLowerCase() + '[name="' + el.name + '"]';
						else if (el.className) info.selector = el.tagName.toLowerCase() + '.' + el.className.split(' ').filter(c => c).join('.');
						return info;
					}
				}
				return null;
			}
		`, elem.Text)

		if ia.pageQueryFunc != nil {
			val, err := ia.pageQueryFunc(script)
			if err == nil {
				if m, ok := val.(map[string]interface{}); ok {
					selector := ""
					if s, ok := m["selector"].(string); ok {
						selector = s
					}
					result = append(result, PageElement{
						Type:     elem.Type,
						Text:     elem.Text,
						Selector: selector,
					})
					continue
				}
			}
		}

		// 如果查询失败，保留原始OCR结果
		result = append(result, elem)
	}

	return result
}

// analyzeWithTencent 使用腾讯云OCR分析图片
func (ia *ImageAnalyzer) analyzeWithTencent(imageBase64, prompt string, cfg config.TencentOCRConfig) (*AnalyzeResult, error) {
	// 腾讯云OCR实现较复杂，需要签名认证
	// 这里先返回未实现
	return nil, fmt.Errorf("腾讯云OCR暂未实现")
}

// analyzeWithCustomAPI 使用自定义API分析图片
func (ia *ImageAnalyzer) analyzeWithCustomAPI(imageBase64, prompt string, cfg config.CustomAPIConfig) (*AnalyzeResult, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("自定义API URL未配置")
	}

	// 构建请求
	reqBody := map[string]interface{}{
		"image": imageBase64,
		"prompt": prompt,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("POST", cfg.URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// 添加自定义headers
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := ia.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("自定义API返回错误: %s, %s", resp.Status, string(body))
	}

	// 尝试解析为AnalyzeResult
	var result AnalyzeResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析自定义API响应失败: %v", err)
	}

	return &result, nil
}

// readImageBase64 读取图片并转换为base64
func readImageBase64(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// imageToBase64 读取图片文件并转换为base64
func imageToBase64(filePath string) (string, error) {
	// 读取图片文件
	imgFile, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer imgFile.Close()

	// 解码图片
	img, _, err := image.Decode(imgFile)
	if err != nil {
		// 如果是PNG解码失败尝试JPEG
		imgFile.Seek(0, 0)
		img, err = png.Decode(imgFile)
		if err != nil {
			return "", fmt.Errorf("无法解码图片: %v", err)
		}
	}

	// 重新编码为PNG并转换为base64
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// FindPhoneInputByAI 使用AI查找手机号输入框
// 返回：CSS选择器, X坐标, Y坐标, 错误
func (ia *ImageAnalyzer) FindPhoneInputByAI(screenshotPath string) (string, int, int, error) {
	result, err := ia.AnalyzePage(screenshotPath, "请找到手机号输入框的位置，返回其CSS选择器和中心坐标")
	if err != nil {
		return "", 0, 0, err
	}

	for _, elem := range result.Elements {
		if strings.Contains(strings.ToLower(elem.Text), "手机") ||
			strings.Contains(strings.ToLower(elem.Description), "手机") ||
			strings.Contains(strings.ToLower(elem.Type), "input") {
			log.Printf("AI找到手机号输入框: selector=%s, text=%s, x=%d, y=%d",
				elem.Selector, elem.Text, elem.X, elem.Y)
			return elem.Selector, elem.X, elem.Y, nil
		}
	}

	return "", 0, 0, fmt.Errorf("AI未能找到手机号输入框")
}

// FindButtonByAI 使用AI查找按钮
// 返回：CSS选择器, X坐标, Y坐标, 错误
func (ia *ImageAnalyzer) FindButtonByAI(screenshotPath string, buttonTexts []string) (string, int, int, error) {
	prompt := fmt.Sprintf("请找到包含以下文本之一的按钮: %s，返回其CSS选择器和中心坐标", strings.Join(buttonTexts, ", "))

	result, err := ia.AnalyzePage(screenshotPath, prompt)
	if err != nil {
		return "", 0, 0, err
	}

	for _, elem := range result.Elements {
		for _, targetText := range buttonTexts {
			if strings.Contains(elem.Text, targetText) || strings.Contains(elem.Description, targetText) {
				log.Printf("AI找到按钮: selector=%s, text=%s, x=%d, y=%d",
					elem.Selector, elem.Text, elem.X, elem.Y)
				return elem.Selector, elem.X, elem.Y, nil
			}
		}
	}

	return "", 0, 0, fmt.Errorf("AI未能找到目标按钮")
}
