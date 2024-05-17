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
				BaseImage: "donknap/dpanel:java-8",
			},
			{
				Name:      "jdk11",
				Version:   "11",
				BaseImage: "donknap/dpanel:java-11",
			},
			{
				Name:      "jdk12",
				Version:   "12",
				BaseImage: "donknap/dpanel:java-12",
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
				BaseImage: "donknap/dpanel:node-12",
			},
			{
				Name:      "node14",
				Version:   "14",
				BaseImage: "donknap/dpanel:node-14",
			},
			{
				Name:      "node16",
				Version:   "16",
				BaseImage: "donknap/dpanel:node-16",
			},
			{
				Name:      "node18",
				Version:   "18",
				BaseImage: "donknap/dpanel:node-18",
			},
		},
	}

	supportEnv[LangHtml] = env{
		Name: LangHtml,
		Env: []envItem{
			{
				Name:      "common",
				Version:   "1.0.0",
				BaseImage: "donknap/dpanel:html-common",
			},
		},
	}
	return supportEnv
}
