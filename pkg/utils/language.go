package utils

type Lang string

const (
	RU Lang = "ru"
	UZ Lang = "uz"
	EN Lang = "en"
)

type Language struct {
	RU string
	UZ string
	EN string
}

func (l Language) By(lang Lang) string {
	if lang == RU {
		return l.RU
	} else if lang == EN {
		return l.EN
	}
	return l.UZ
}

func ParseLang(lang string) (Lang, bool) {
	switch lang {
	case "ru", "RU":
		return RU, true
	case "uz", "UZ":
		return UZ, true
	case "en", "EN":
		return EN, true
	default:
		return "", false
	}
}
