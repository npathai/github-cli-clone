package github

func newConfigService() *configService {
	return &configService{
		Encoder: &yamlConfigEncoder{},
		Decoder: &yamlConfigDecoder{},
	}
}

type configService struct {
	Encoder configEncoder
	Decoder configDecoder
}
