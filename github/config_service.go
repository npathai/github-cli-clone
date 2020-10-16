package github

import "os"

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

func (service *configService) Load(filename string, config *Config) error {
	r, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer r.Close()

	return service.Decoder.Decode(r, config)
}