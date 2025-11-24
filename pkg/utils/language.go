package utils

type Lang string

const (
	RU Lang = "ru"
	UZ Lang = "uz"
)

type Language struct {
	RU string
	UZ string
}

func (l Language) By(lang Lang) string {
	if lang == RU {
		return l.RU
	}
	return l.UZ
}

func ParseLang(lang string) (Lang, bool) {
	switch lang {
	case "ru", "RU":
		return RU, true
	case "uz", "UZ":
		return UZ, true
	default:
		return "", false
	}
}
