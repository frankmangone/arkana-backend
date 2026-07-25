package models

// WriterPayload is the body of POST /api/admin/writers. Slug, Name,
// ImageURL, and AvatarURL are required; everything else is optional.
type WriterPayload struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	ImageURL      string            `json:"image_url"`
	AvatarURL     string            `json:"avatar_url"`
	Visible       *bool             `json:"visible,omitempty"`
	Organization  *Organization     `json:"organization,omitempty"`
	Bio           map[string]string `json:"bio,omitempty"`
	Social        *Social           `json:"social,omitempty"`
	WalletAddress string            `json:"wallet_address,omitempty"`
}

type Organization struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	LogoURL string `json:"logo_url,omitempty"`
}

type Social struct {
	Website  string `json:"website,omitempty"`
	Twitter  string `json:"twitter,omitempty"`
	LinkedIn string `json:"linkedin,omitempty"`
	GitHub   string `json:"github,omitempty"`
	Email    string `json:"email,omitempty"`
	Medium   string `json:"medium,omitempty"`
}

type PublishWriterResponse struct {
	Published bool `json:"published"`
}

type WriterResponse struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	ImageURL      string            `json:"image_url"`
	AvatarURL     string            `json:"avatar_url"`
	Organization  *Organization     `json:"organization,omitempty"`
	Bio           map[string]string `json:"bio,omitempty"`
	Social        *Social           `json:"social,omitempty"`
	WalletAddress string            `json:"wallet_address,omitempty"`
}

type WriterSummary struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	ImageURL  string `json:"image_url"`
	AvatarURL string `json:"avatar_url"`
}

// WriterListResponse wraps GET /api/writers's list in a data key so
// pagination metadata (page, total, ...) can be added later without a
// breaking shape change.
type WriterListResponse struct {
	Data []WriterSummary `json:"data"`
}

// AdminWriterListResponse wraps GET /api/admin/writers's list in a data key,
// same rationale as WriterListResponse.
type AdminWriterListResponse struct {
	Data []WriterResponse `json:"data"`
}
