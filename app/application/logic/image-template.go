package logic

type ImageTemplate struct {
}

type envItem struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	BaseImage string `json:"baseImage"`
}

type env struct {
	Name string    `json:"name"`
	Env  []envItem `json:"env"`
	Ext  []string  `json:"ext"`
}

func (self ImageTemplate) GetSupportEnv() map[string]env {
	supportEnv := make(map[string]env)

	supportEnv[LangPhp] = env{
		Name: LangPhp,
		Env: []envItem{
			{
				Name:      "php-72",
				BaseImage: "donknap/dpanel:php-72|7",
			},
			{
				Name:      "php-74",
				BaseImage: "donknap/dpanel:php-74|7",
			},
			{
				Name:      "php-81",
				BaseImage: "donknap/dpanel:php-81|81",
			},
		},
		Ext: []string{
			"intl", "pecl-apcu", "imap", "pecl-mongodb", "pdo_pgsql",
		},
	}

	supportEnv[LangJava] = env{
		Name: LangJava,
		Env: []envItem{
			{
				Name:      "jdk8",
				Version:   "8",
				BaseImage: "alpine:3.18",
			},
			{
				Name:      "jdk12",
				Version:   "12",
				BaseImage: "alpine:3.18",
			},
			{
				Name:      "jdk18",
				Version:   "18",
				BaseImage: "alpine:3.18",
			},
		},
	}

	supportEnv[LangGolang] = env{
		Name: LangGolang,
		Env: []envItem{
			{
				Name:      "go1.21",
				Version:   "1.21",
				BaseImage: "donknap/dpanel:go-1.21|1.21",
			},
		},
	}

	supportEnv[LangNode] = env{
		Name: LangNode,
		Env: []envItem{
			{
				Name:      "node12",
				Version:   "12",
				BaseImage: "node:12.22.12-alpine",
			},
			{
				Name:      "node14",
				Version:   "14",
				BaseImage: "node:14.21-alpine",
			},
			{
				Name:      "node18",
				Version:   "18",
				BaseImage: "node:18.19-alpine",
			},
			{
				Name:      "node20",
				Version:   "20",
				BaseImage: "node:20.11-alpine",
			},
		},
	}

	supportEnv[LangHtml] = env{
		Name: LangHtml,
		Env: []envItem{
			{
				Name:      "universal",
				Version:   "1.0.0",
				BaseImage: "alpine:3.18",
			},
		},
	}
	return supportEnv
}
