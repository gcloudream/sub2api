package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type codexImageGenerationLiteCompatibilityContextKey struct{}

func codexImageGenerationLiteCompatibilityEnabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	enabled, _ := ctx.Value(codexImageGenerationLiteCompatibilityContextKey{}).(bool)
	return enabled
}

// prepareCodexImageGenerationLiteCompatibility opts a single image-request
// attempt out of Responses Lite. Ingress headers and the caller's bytes remain
// unchanged so a scheduler retry can evaluate its own account policy.
func (s *OpenAIGatewayService) prepareCodexImageGenerationLiteCompatibility(ctx context.Context, c *gin.Context, account *Account, body []byte) (context.Context, []byte, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// A derived context must not carry a prior account's opt-in past failover.
	if codexImageGenerationLiteCompatibilityEnabled(ctx) {
		ctx = context.WithValue(ctx, codexImageGenerationLiteCompatibilityContextKey{}, false)
	}
	if s == nil || c == nil || c.Request == nil || account == nil ||
		!account.IsOpenAIOAuthLike() || account.IsOpenAIPassthroughEnabled() ||
		isOpenAIResponsesCompactPath(c) ||
		account.CodexImageGenerationExplicitToolPolicy() == codexImageGenerationExplicitToolPolicyStrip {
		return ctx, body, false, nil
	}
	isCodex := openai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) ||
		(s.cfg != nil && s.cfg.Gateway.ForceCodexCLI)
	apiKey := getAPIKeyFromContext(c)
	if !isCodex || !GroupAllowsImageGeneration(apiKeyGroup(apiKey)) || !s.isCodexImageGenerationBridgeEnabled(ctx, account, apiKey) {
		return ctx, body, false, nil
	}
	if !isOpenAIResponsesLiteHeader(c.GetHeader(responsesLiteHeader)) && !isOpenAIResponsesLiteWebSocketPayload(body) {
		return ctx, body, false, nil
	}
	if !gjson.ValidBytes(body) {
		return ctx, body, false, fmt.Errorf("invalid JSON in Codex image Lite compatibility request")
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if isCodexSparkModel(model) || isCodexSparkModel(account.GetMappedModel(model)) || !codexImageLiteToolChoiceAllowsImage(body) {
		return ctx, body, false, nil
	}
	text, hasImage := codexImageLiteLatestUserText(body)
	if !IsExplicitImageGenerationIntent(openAIResponsesEndpoint, model, body) && !codexImageLiteHasDirectImageRequest(text, hasImage) {
		return ctx, body, false, nil
	}
	normalized, err := removeCodexImageLiteMetadata(body)
	if err != nil {
		return ctx, body, false, err
	}
	ctx = context.WithValue(ctx, codexImageGenerationLiteCompatibilityContextKey{}, true)
	logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Codex image Lite compatibility enabled")
	return ctx, normalized, true, nil
}

func codexImageLiteToolChoiceAllowsImage(body []byte) bool {
	choice := gjson.GetBytes(body, "tool_choice")
	if !choice.Exists() || choice.Type == gjson.Null {
		return true
	}
	if choice.Type == gjson.String {
		switch strings.ToLower(strings.TrimSpace(choice.String())) {
		case "auto", "required":
			return true
		default:
			return false
		}
	}
	return choice.IsObject() && strings.TrimSpace(choice.Get("type").String()) == "image_generation"
}

// The most recent user message is the only language signal. Tool catalogs,
// developer instructions, image URLs and older user turns are not requests.
func codexImageLiteLatestUserText(body []byte) (string, bool) {
	input := gjson.GetBytes(body, "input")
	if input.Type == gjson.String {
		return input.String(), false
	}
	items := input.Array()
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if strings.TrimSpace(item.Get("role").String()) != "user" ||
			(item.Get("type").Exists() && item.Get("type").String() != "message") {
			continue
		}
		content := item.Get("content")
		if content.Type == gjson.String {
			return content.String(), false
		}
		var texts []string
		hasImage := false
		for _, part := range content.Array() {
			switch part.Get("type").String() {
			case "input_text", "text":
				if value := part.Get("text"); value.Type == gjson.String {
					texts = append(texts, value.String())
				}
			case "input_image", "image_url":
				hasImage = true
			}
		}
		return strings.Join(texts, "\n"), hasImage
	}
	return "", false
}

var (
	codexImageLiteSkillPrefix     = regexp.MustCompile(`^\$imagegen(?:\s+|[：:]+\s*)`)
	codexImageLiteDirectStart     = regexp.MustCompile(`(?i)^(?:(?:请你|请|麻烦你|麻烦|帮我|帮忙|给我|为我|替我|直接|现在|立即|马上)\s*)*(生成|绘制|制作|创建|设计|创作|画|编辑|修改|重绘|修复|美化|扩展|放大|裁剪|旋转|重新画|重新绘制|把|将|给|去掉|移除|删除|替换|更换|去除)|^(?:(?:please|kindly|(?:can|could|would) you)\s+)*(generate|create|make|render|draw|produce|design|paint|illustrate|edit|modify|retouch|repaint|inpaint|outpaint|upscale|resize|crop|rotate|recreate|redraw|change|replace|remove|add)\b`)
	codexImageLiteTextArtifact    = regexp.MustCompile(`(?i)^(?:(?:请你|请|麻烦你|麻烦|帮我|帮忙|给我|为我|替我|直接|现在|立即|马上)\s*)*(?:生成|创建|制作|设计|创作)(?:一段|一篇|一句|段|篇|句|(?:一个|一份)?(?:文章|段落|说明文字|标题|句子|描述|提示词))|^(?:(?:please|kindly|(?:can|could|would) you)\s+)*(?:generate|create|make|produce|design)\s+(?:(?:a|an|the|one|some)\s+)?(?:paragraphs?|sentences?|titles?|articles?|descriptions?|captions?|essays?|summaries|summary|stories|story|prompts?)\b`)
	codexImageLiteTechnicalObject = regexp.MustCompile(`(?i)(图片|图像|照片|image|picture|photo).{0,24}(上传组件|组件|控件|接口|函数|逻辑|属性|字段|文件名|metadata|json|alt\b)|\b(edit|modify|change|create|generate|build)\b[^\n.!?]{0,60}\b(components?|handlers?|attributes?|propert(?:y|ies)|metadata|json)\b`)
	codexImageLiteNegative        = regexp.MustCompile(`(?i)(不要|不用|无需|不需要|不必|禁止|不许|别|不)(再|真的|实际|直接|为我|帮我|替我|现在|去|\s)*(生成|生图|画|绘制|绘画|制作|创建|创作|编辑|修改|重绘|出图)|\b(do not|don't|dont|never|no need to|avoid|without)\s+(actually\s+)?(generate|create|make|draw|paint|render|edit|modify|retouch)\b`)
	codexImageLiteExplanation     = regexp.MustCompile(`(?i)^\s*(?:(请|你|您|帮我|帮忙|麻烦|给我|为我|替我|先)\s*)*(分析|解释|讲解|讨论|评估|研究|说明|告诉我|教我|介绍|比较|查询|确认|检查|能否|能不能|会不会|是否支持|支持不支持|可不可以|请问|想问|我想知道|我想了解)|^\s*(please\s+)?(explain|describe|analy[sz]e|assess|discuss|evaluate|tell me|show me how|how|what|why|whether)\b`)
	codexImageLiteCapability      = regexp.MustCompile(`(?i)^\s*(你|您|它|这个模型|该模型|当前模型|幻想家)?\s*(可以|能|支持|会)\s*(生图|出图|生成图片|生成图像|生成照片|画图|绘画|编辑图片|修改图片)\s*(吗|么|不|[?？])?[?？.!！。\s]*$|^\s*(can|could|do|does|will|would|are|is)\b.{0,35}\b(generate|draw|create|make|edit|support)\s+(an?\s+)?(images?|pictures?|photos?|image generation)[?!.\s]*$`)
	codexImageLiteCode            = regexp.MustCompile(`(?i)(编写|撰写|写|实现|开发|调试|修复|配置|接入|部署|测试|优化|解释|分析|评估|讨论|介绍|提供)[^\n。！？.!?；;，,]{0,28}(代码|脚本|接口|api\b|sdk\b|程序|函数|功能|网页|网站|应用|客户端|服务端|教程|文档|方案|提示词)|\b(write|implement|develop|debug|fix|configure|integrate|deploy|test|explain|build)\b[^\n。！？.!?；;，,]{0,50}\b(code|scripts?|api|sdk|functions?|programs?|websites?|apps?|applications?|tutorials?|documentation|prompts?)\b|(生成|创建|设计)(一个|一段|一份|个|一套)?(生图|图片生成|图片|图像|绘画)?(代码|脚本|接口|功能|文档|方案|提示词)|\b(create|generate|design)\s+(an?\s+|the\s+)?((python|javascript|typescript|shell|bash|java|go)\s+)?(code|script|api|sdk|function|program|website|app|application|tutorial|documentation|prompt)\b`)
	codexImageLiteNegatedCode     = regexp.MustCompile(`(?i)(不要|不用|无需|不必|不需要|别|禁止|不许)\s*$|\b(do not|don't|dont|without|no need to|avoid)\s*$`)
	codexImageLiteChineseGenerate = regexp.MustCompile(`(绘制|画)(出|成|一张|一幅|一只|一个|一条|一头|一匹|几张|多张|张|幅|个|只|一下|些|一组|一套)|(?:生成|绘制|制作|创建|设计|创作|画).{0,48}(图片|图像|照片|插画|海报|壁纸|头像|图标|肖像|标志|logo)`)
	codexImageLiteChineseEdit     = regexp.MustCompile(`(编辑|修改|重绘|修复|美化|扩展|放大|裁剪|旋转|重新画|重新绘制).{0,20}(这张图|那张图|此图|原图|图片|图像|照片|插画|海报|头像)|(?:把|将|给).{0,12}(这张图|那张图|此图|原图|图片|图像|照片|插画|海报|头像).{0,36}(改成|换成|替换|去掉|移除|删除|修改|编辑|添加|加上|去除)`)
	codexImageLiteEnglishGenerate = regexp.MustCompile(`(?i)\b(generate|create|make|render|draw|produce|design|paint|illustrate)\b[^\n.!?]{0,60}\b(image|picture|photo|illustration|portrait|poster|icon|logo|wallpaper|artwork|drawing|painting)\b|^\s*(please\s+)?((can|could|would) you\s+)?(draw|paint|illustrate)\s+\S+`)
	codexImageLiteEnglishEdit     = regexp.MustCompile(`(?i)\b(edit|modify|retouch|repaint|inpaint|outpaint|upscale|resize|crop|rotate|recreate|redraw)\b[^\n.!?]{0,50}\b(image|picture|photo|illustration|portrait|poster|icon|logo|wallpaper|artwork|drawing|painting)\b|\b(change|replace|remove|add)\b[^\n.!?]{0,45}\b(this|attached|uploaded|my)\s+(image|picture|photo)\b`)
	codexImageLiteAttachedEdit    = regexp.MustCompile(`(?i)(把|将).{0,24}(背景|颜色|风格|文字|人物|衣服).{0,16}(改成|换成|替换|去掉|移除|删除|加上|去除)|(?:去掉|移除|删除|替换|更换|修改|编辑|去除).{0,12}(背景|颜色|风格|文字|人物)|\b(remove|replace|change|edit|modify|retouch|crop|upscale|resize)\b[^\n.!?]{0,40}\b(background|colou?r|style|text|person|people)\b`)
)

func codexImageLiteHasDirectImageRequest(text string, hasImage bool) bool {
	text = strings.TrimSpace(text)
	text = strings.TrimSpace(codexImageLiteSkillPrefix.ReplaceAllString(text, ""))
	if text == "" || len(text) > 16*1024 || strings.Contains(text, "```") ||
		!codexImageLiteDirectStart.MatchString(text) || codexImageLiteTextArtifact.MatchString(text) || codexImageLiteTechnicalObject.MatchString(text) ||
		codexImageLiteExplanation.MatchString(text) || codexImageLiteCapability.MatchString(text) {
		return false
	}
	firstRequest := -1
	patterns := []*regexp.Regexp{codexImageLiteChineseGenerate, codexImageLiteEnglishGenerate}
	if hasImage {
		patterns = append(patterns, codexImageLiteChineseEdit, codexImageLiteEnglishEdit, codexImageLiteAttachedEdit)
	}
	for _, pattern := range patterns {
		if span := pattern.FindStringIndex(text); span != nil && (firstRequest < 0 || span[0] < firstRequest) {
			firstRequest = span[0]
		}
	}
	if firstRequest < 0 {
		return false
	}
	if negative := codexImageLiteNegative.FindStringIndex(text); negative != nil && negative[0] <= firstRequest {
		return false
	}
	for _, span := range codexImageLiteCode.FindAllStringIndex(text, -1) {
		// "Generate a picture; do not write code" still requests a picture.
		prefix := text[:span[0]]
		if !codexImageLiteNegatedCode.MatchString(prefix) {
			return false
		}
	}
	return true
}

func removeCodexImageLiteMetadata(body []byte) ([]byte, error) {
	// sjson keeps unrelated JSON (notably 64-bit IDs and tool grammars) intact.
	if !gjson.GetBytes(body, "client_metadata."+responsesLiteWSMetadataKey).Exists() {
		return body, nil
	}
	normalized, err := sjson.DeleteBytes(body, "client_metadata."+responsesLiteWSMetadataKey)
	if err != nil {
		return nil, fmt.Errorf("remove Codex image Lite metadata: %w", err)
	}
	return normalized, nil
}

// applyCodexImageGenerationLiteCompatibility runs after upstream headers have
// been copied. A replayable body lets this boundary update Body and GetBody
// together without consuming a caller-owned streaming body on failure.
func applyCodexImageGenerationLiteCompatibility(req *http.Request) error {
	if req == nil || !codexImageGenerationLiteCompatibilityEnabled(req.Context()) {
		return nil
	}
	var normalized []byte
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody == nil {
			return fmt.Errorf("Codex image Lite compatibility requires a replayable request body")
		}
		bodyReader, err := req.GetBody()
		if err != nil {
			return fmt.Errorf("replay Codex image Lite request body: %w", err)
		}
		body, readErr := io.ReadAll(bodyReader)
		closeErr := bodyReader.Close()
		if readErr != nil {
			return fmt.Errorf("read Codex image Lite request body: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close Codex image Lite request body: %w", closeErr)
		}
		if !gjson.ValidBytes(body) {
			return fmt.Errorf("invalid JSON at Codex image Lite request boundary")
		}
		normalized, err = removeCodexImageLiteMetadata(body)
		if err != nil {
			return err
		}
	}
	if normalized != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		body := append([]byte(nil), normalized...)
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	}
	req.Header.Del(responsesLiteHeader)
	return nil
}
