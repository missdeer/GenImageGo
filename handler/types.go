package handler

type geminiRequest struct {
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenerationConfig struct {
	ResponseMIMEType string `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates,omitempty"`
	Error      *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content *geminiContent `json:"content,omitempty"`
}

type geminiError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
