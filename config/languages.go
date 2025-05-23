package config

type Language struct {
	Code string
	Name string
}

// 支持的语言列表
var SupportedLanguagesAli = []Language{
	{Code: "ab", Name: "Abkhazian"},
	{Code: "sq", Name: "Albanian"},
	{Code: "ak", Name: "Akan"},
	{Code: "ar", Name: "Arabic"},
	{Code: "an", Name: "Aragonese"},
	{Code: "am", Name: "Amharic"},
	{Code: "as", Name: "Assamese"},
	{Code: "az", Name: "Azerbaijani"},
	{Code: "ast", Name: "Asturian"},
	{Code: "nch", Name: "Central Huasteca Nahuatl"},
	{Code: "ee", Name: "Ewe"},
	{Code: "ay", Name: "Aymara"},
	{Code: "ga", Name: "Irish"},
	{Code: "et", Name: "Estonian"},
	{Code: "oj", Name: "Ojibwa"},
	{Code: "oc", Name: "Occitan"},
	{Code: "or", Name: "Oriya"},
	{Code: "om", Name: "Oromo"},
	{Code: "os", Name: "Ossetian"},
	{Code: "tpi", Name: "Tok Pisin"},
	{Code: "ba", Name: "Bashkir"},
	{Code: "eu", Name: "Basque"},
	{Code: "be", Name: "Belarusian"},
	{Code: "ber", Name: "Berber languages"},
	{Code: "bm", Name: "Bambara"},
	{Code: "pag", Name: "Pangasinan"},
	{Code: "bg", Name: "Bulgarian"},
	{Code: "se", Name: "Northern Sami"},
	{Code: "bem", Name: "Bemba (Zambia)"},
	{Code: "byn", Name: "Blin"},
	{Code: "bi", Name: "Bislama"},
	{Code: "bal", Name: "Baluchi"},
	{Code: "is", Name: "Icelandic"},
	{Code: "pl", Name: "Polish"},
	{Code: "bs", Name: "Bosnian"},
	{Code: "fa", Name: "Persian"},
	{Code: "bho", Name: "Bhojpuri"},
	{Code: "br", Name: "Breton"},
	{Code: "ch", Name: "Chamorro"},
	{Code: "cbk", Name: "Chavacano"},
	{Code: "cv", Name: "Chuvash"},
	{Code: "ts", Name: "Tsonga"},
	{Code: "tt", Name: "Tatar"},
	{Code: "da", Name: "Danish"},
	{Code: "shn", Name: "Shan"},
	{Code: "tet", Name: "Tetum"},
	{Code: "de", Name: "German"},
	{Code: "nds", Name: "Low German"},
	{Code: "sco", Name: "Scots"},
	{Code: "dv", Name: "Dhivehi"},
	{Code: "kdx", Name: "Kam"},
	{Code: "dtp", Name: "Kadazan Dusun"},
	{Code: "ru", Name: "Russian"},
	{Code: "fo", Name: "Faroese"},
	{Code: "fr", Name: "French"},
	{Code: "sa", Name: "Sanskrit"},
	{Code: "fil", Name: "Filipino"},
	{Code: "fj", Name: "Fijian"},
	{Code: "fi", Name: "Finnish"},
	{Code: "fur", Name: "Friulian"},
	{Code: "fvr", Name: "Fur"},
	{Code: "kg", Name: "Kongo"},
	{Code: "km", Name: "Khmer"},
	{Code: "ngu", Name: "Guerrero Nahuatl"},
	{Code: "kl", Name: "Kalaallisut"},
	{Code: "ka", Name: "Georgian"},
	{Code: "gos", Name: "Gronings"},
	{Code: "gu", Name: "Gujarati"},
	{Code: "gn", Name: "Guarani"},
	{Code: "kk", Name: "Kazakh"},
	{Code: "ht", Name: "Haitian"},
	{Code: "ko", Name: "Korean"},
	{Code: "ha", Name: "Hausa"},
	{Code: "nl", Name: "Dutch"},
	{Code: "cnr", Name: "Montenegrin"},
	{Code: "hup", Name: "Hupa"},
	{Code: "gil", Name: "Gilbertese"},
	{Code: "rn", Name: "Rundi"},
	{Code: "quc", Name: "K'iche'"},
	{Code: "ky", Name: "Kirghiz"},
	{Code: "gl", Name: "Galician"},
	{Code: "ca", Name: "Catalan"},
	{Code: "cs", Name: "Czech"},
	{Code: "kab", Name: "Kabyle"},
	{Code: "kn", Name: "Kannada"},
	{Code: "kr", Name: "Kanuri"},
	{Code: "csb", Name: "Kashubian"},
	{Code: "kha", Name: "Khasi"},
	{Code: "kw", Name: "Cornish"},
	{Code: "xh", Name: "Xhosa"},
	{Code: "co", Name: "Corsican"},
	{Code: "mus", Name: "Creek"},
	{Code: "crh", Name: "Crimean Tatar"},
	{Code: "tlh", Name: "Klingon"},
	{Code: "hbs", Name: "Serbo-Croatian"},
	{Code: "qu", Name: "Quechua"},
	{Code: "ks", Name: "Kashmiri"},
	{Code: "ku", Name: "Kurdish"},
	{Code: "la", Name: "Latin"},
	{Code: "ltg", Name: "Latgalian"},
	{Code: "lv", Name: "Latvian"},
	{Code: "lo", Name: "Lao"},
	{Code: "lt", Name: "Lithuanian"},
	{Code: "li", Name: "Limburgish"},
	{Code: "ln", Name: "Lingala"},
	{Code: "lg", Name: "Ganda"},
	{Code: "lb", Name: "Letzeburgesch"},
	{Code: "rue", Name: "Rusyn"},
	{Code: "rw", Name: "Kinyarwanda"},
	{Code: "ro", Name: "Romanian"},
	{Code: "rm", Name: "Romansh"},
	{Code: "rom", Name: "Romany"},
	{Code: "jbo", Name: "Lojban"},
	{Code: "mg", Name: "Malagasy"},
	{Code: "gv", Name: "Manx"},
	{Code: "mt", Name: "Maltese"},
	{Code: "mr", Name: "Marathi"},
	{Code: "ml", Name: "Malayalam"},
	{Code: "ms", Name: "Malay"},
	{Code: "chm", Name: "Mari (Russia)"},
	{Code: "mk", Name: "Macedonian"},
	{Code: "mh", Name: "Marshallese"},
	{Code: "kek", Name: "Kekchí"},
	{Code: "mai", Name: "Maithili"},
	{Code: "mfe", Name: "Morisyen"},
	{Code: "mi", Name: "Maori"},
	{Code: "mn", Name: "Mongolian"},
	{Code: "bn", Name: "Bengali"},
	{Code: "my", Name: "Burmese"},
	{Code: "hmn", Name: "Hmong"},
	{Code: "umb", Name: "Umbundu"},
	{Code: "nv", Name: "Navajo"},
	{Code: "af", Name: "Afrikaans"},
	{Code: "ne", Name: "Nepali"},
	{Code: "niu", Name: "Niuean"},
	{Code: "no", Name: "Norwegian"},
	{Code: "pmn", Name: "Pam"},
	{Code: "pap", Name: "Papiamento"},
	{Code: "pa", Name: "Panjabi"},
	{Code: "pt", Name: "Portuguese"},
	{Code: "ps", Name: "Pushto"},
	{Code: "ny", Name: "Nyanja"},
	{Code: "tw", Name: "Twi"},
	{Code: "chr", Name: "Cherokee"},
	{Code: "ja", Name: "Japanese"},
	{Code: "sv", Name: "Swedish"},
	{Code: "sm", Name: "Samoan"},
	{Code: "sg", Name: "Sango"},
	{Code: "si", Name: "Sinhala"},
	{Code: "hsb", Name: "Upper Sorbian"},
	{Code: "eo", Name: "Esperanto"},
	{Code: "sl", Name: "Slovenian"},
	{Code: "sw", Name: "Swahili"},
	{Code: "so", Name: "Somali"},
	{Code: "sk", Name: "Slovak"},
	{Code: "tl", Name: "Tagalog"},
	{Code: "tg", Name: "Tajik"},
	{Code: "ty", Name: "Tahitian"},
	{Code: "te", Name: "Telugu"},
	{Code: "ta", Name: "Tamil"},
	{Code: "th", Name: "Thai"},
	{Code: "to", Name: "Tonga (Tonga Islands)"},
	{Code: "toi", Name: "Tonga (Zambia)"},
	{Code: "ti", Name: "Tigrinya"},
	{Code: "tvl", Name: "Tuvalu"},
	{Code: "tyv", Name: "Tuvinian"},
	{Code: "tr", Name: "Turkish"},
	{Code: "tk", Name: "Turkmen"},
	{Code: "wa", Name: "Walloon"},
	{Code: "war", Name: "Waray (Philippines)"},
	{Code: "cy", Name: "Welsh"},
	{Code: "ve", Name: "Venda"},
	{Code: "vo", Name: "Volapük"},
	{Code: "wo", Name: "Wolof"},
	{Code: "udm", Name: "Udmurt"},
	{Code: "ur", Name: "Urdu"},
	{Code: "uz", Name: "Uzbek"},
	{Code: "es", Name: "Spanish"},
	{Code: "ie", Name: "Interlingue"},
	{Code: "fy", Name: "Western Frisian"},
	{Code: "szl", Name: "Silesian"},
	{Code: "he", Name: "Hebrew"},
	{Code: "hil", Name: "Hiligaynon"},
	{Code: "haw", Name: "Hawaiian"},
	{Code: "el", Name: "Modern Greek"},
	{Code: "lfn", Name: "Lingua Franca Nova"},
	{Code: "sd", Name: "Sindhi"},
	{Code: "hu", Name: "Hungarian"},
	{Code: "sn", Name: "Shona"},
	{Code: "ceb", Name: "Cebuano"},
	{Code: "syr", Name: "Syriac"},
	{Code: "su", Name: "Sundanese"},
	{Code: "hy", Name: "Armenian"},
	{Code: "ace", Name: "Achinese"},
	{Code: "iba", Name: "Iban"},
	{Code: "ig", Name: "Igbo"},
	{Code: "io", Name: "Ido"},
	{Code: "ilo", Name: "Iloko"},
	{Code: "iu", Name: "Inuktitut"},
	{Code: "it", Name: "Italian"},
	{Code: "yi", Name: "Yiddish"},
	{Code: "ia", Name: "Interlingua"},
	{Code: "hi", Name: "Hindi"},
	{Code: "id", Name: "Indonesia"},
	{Code: "inh", Name: "Ingush"},
	{Code: "en", Name: "English"},
	{Code: "yo", Name: "Yoruba"},
	{Code: "vi", Name: "Vietnamese"},
	{Code: "zza", Name: "Zaza"},
	{Code: "jv", Name: "Javanese"},
	{Code: "zh", Name: "Chinese"},
	{Code: "zh-tw", Name: "Traditional Chinese"},
	{Code: "yue", Name: "Cantonese"},
	{Code: "zu", Name: "Zulu"},
}

// GetLanguageByCode 根据语言代码获取语言名称
func GetLanguageByCode(code string) (string, bool) {
	for _, lang := range SupportedLanguagesAli {
		if lang.Code == code {
			return lang.Name, true
		}
	}
	return "", false
}

// IsLanguageSupported 检查语言是否支持
func IsLanguageSupported(code string) bool {
	_, exists := GetLanguageByCode(code)
	return exists
}
