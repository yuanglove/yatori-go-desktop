package service

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------- 配置结构（与 yatori-go-console config.yaml 完全对应）----------

type AppConfig struct {
	Setting Setting `yaml:"setting" json:"setting"`
	Users   []User  `yaml:"users" json:"users"`
}

type Setting struct {
	BasicSetting  BasicSetting  `yaml:"basicSetting" json:"basicSetting"`
	EmailInform   EmailInform   `yaml:"emailInform" json:"emailInform"`
	AiSetting     AiSetting     `yaml:"aiSetting" json:"aiSetting"`
	ApiQueSetting ApiQueSetting `yaml:"apiQueSetting" json:"apiQueSetting"`
}

type BasicSetting struct {
	CompletionTone int    `yaml:"completionTone" json:"completionTone"`
	ColorLog       int    `yaml:"colorLog" json:"colorLog"`
	LogOutFileSw   int    `yaml:"logOutFileSw" json:"logOutFileSw"`
	LogLevel       string `yaml:"logLevel" json:"logLevel"`
	LogModel       int    `yaml:"logModel" json:"logModel"`
	WebModel       int    `yaml:"webModel" json:"webModel"`
	Theme          string `yaml:"theme,omitempty" json:"theme,omitempty"`
}

type EmailInform struct {
	Sw       int    `yaml:"sw" json:"sw"`
	SMTPHost string `yaml:"SMTPHost" json:"smtpHost"`
	SMTPPort int    `yaml:"SMTPPort" json:"smtpPort"`
	UserName string `yaml:"userName" json:"userName"`
	Password string `yaml:"password" json:"password"`
}

type AiSetting struct {
	AiType string `yaml:"aiType" json:"aiType"`
	AiUrl  string `yaml:"aiUrl" json:"aiUrl"`
	Model  string `yaml:"model" json:"model"`
	APIKey string `yaml:"API_KEY" json:"apiKey"`
}

type ApiQueSetting struct {
	Url             string `yaml:"url" json:"url"`
	Protocol        string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	Token           string `yaml:"token,omitempty" json:"token,omitempty"`
	TokenParam      string `yaml:"tokenParam,omitempty" json:"tokenParam,omitempty"`
	AuthType        string `yaml:"authType,omitempty" json:"authType,omitempty"`
	Method          string `yaml:"method,omitempty" json:"method,omitempty"`
	ContentType     string `yaml:"contentType,omitempty" json:"contentType,omitempty"`
	QuestionField   string `yaml:"questionField,omitempty" json:"questionField,omitempty"`
	TypeField       string `yaml:"typeField,omitempty" json:"typeField,omitempty"`
	OptionsField    string `yaml:"optionsField,omitempty" json:"optionsField,omitempty"`
	CourseNameField string `yaml:"courseNameField,omitempty" json:"courseNameField,omitempty"`
	OptionsFormat   string `yaml:"optionsFormat,omitempty" json:"optionsFormat,omitempty"`
	TypeMap         string `yaml:"typeMap,omitempty" json:"typeMap,omitempty"`
	AnswerPath      string `yaml:"answerPath,omitempty" json:"answerPath,omitempty"`
	AnswerSplit     string `yaml:"answerSplit,omitempty" json:"answerSplit,omitempty"`
}

type User struct {
	AccountType   string        `yaml:"accountType" json:"accountType"`
	URL           string        `yaml:"url" json:"url"`
	RemarkName    string        `yaml:"remarkName,omitempty" json:"remarkName,omitempty"`
	Account       string        `yaml:"account" json:"account"`
	Password      string        `yaml:"password" json:"password"`
	IsProxy       int           `yaml:"isProxy" json:"isProxy"`
	InformEmails  []string      `yaml:"informEmails" json:"informEmails"`
	CoursesCustom CoursesCustom `yaml:"coursesCustom" json:"coursesCustom"`
}

type CoursesCustom struct {
	StudyTime              string            `yaml:"studyTime,omitempty" json:"studyTime,omitempty"`
	CxNode                 *int              `yaml:"cxNode,omitempty" json:"cxNode,omitempty"`
	CxChapterTestSw        *int              `yaml:"cxChapterTestSw,omitempty" json:"cxChapterTestSw,omitempty"`
	CxWorkSw               *int              `yaml:"cxWorkSw,omitempty" json:"cxWorkSw,omitempty"`
	CxExamSw               *int              `yaml:"cxExamSw,omitempty" json:"cxExamSw,omitempty"`
	ShuffleSw              int               `yaml:"shuffleSw" json:"shuffleSw"`
	VideoModel             int               `yaml:"videoModel" json:"videoModel"`
	AutoExam               int               `yaml:"autoExam" json:"autoExam"`
	ExamAutoSubmit         int               `yaml:"examAutoSubmit" json:"examAutoSubmit"`
	SubmitThresholdPercent int               `yaml:"submitThresholdPercent,omitempty" json:"submitThresholdPercent,omitempty"`
	RandomAnswerOnFail     int               `yaml:"randomAnswerOnFail,omitempty" json:"randomAnswerOnFail,omitempty"`
	XxtAutoExamCode        *int              `yaml:"xxtAutoExamCode,omitempty" json:"xxtAutoExamCode,omitempty"`
	XxtExamCodes           map[string]string `yaml:"xxtExamCodes,omitempty" json:"xxtExamCodes,omitempty"`
	ExcludeCourses         []string          `yaml:"excludeCourses" json:"excludeCourses"`
	IncludeCourses         []string          `yaml:"includeCourses" json:"includeCourses"`
	CoursesSettings        []CourseSettings  `yaml:"coursesSettings,omitempty" json:"coursesSettings,omitempty"`
}

type CourseSettings struct {
	Name         string   `yaml:"name" json:"name"`
	IncludeExams []string `yaml:"includeExams,omitempty" json:"includeExams,omitempty"`
	ExcludeExams []string `yaml:"excludeExams,omitempty" json:"excludeExams,omitempty"`
}

// ---------- 读写 ----------

func LoadConfig(path string) (AppConfig, error) {
	var cfg AppConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return cfg, fmt.Errorf("读取配置文件失败: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析 YAML 失败: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func SaveConfig(path string, cfg AppConfig) error {
	applyDefaults(&cfg)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func ValidateConfig(cfg AppConfig) []string {
	var errs []string
	level := strings.ToUpper(cfg.Setting.BasicSetting.LogLevel)
	if level != "INFO" && level != "DEBUG" && level != "WARN" && level != "ERROR" {
		errs = append(errs, "logLevel 必须是 INFO / DEBUG / WARN / ERROR 之一")
	}
	if cfg.Setting.EmailInform.Sw == 1 {
		if cfg.Setting.EmailInform.SMTPHost == "" {
			errs = append(errs, "开启邮件通知时 SMTP Host 不能为空")
		}
		if cfg.Setting.EmailInform.SMTPPort <= 0 {
			errs = append(errs, "开启邮件通知时 SMTP Port 必须大于 0")
		}
	}
	for i, u := range cfg.Users {
		if u.Account == "" {
			errs = append(errs, fmt.Sprintf("第 %d 个账号的 account 不能为空", i+1))
		}
		if u.AccountType == "" {
			errs = append(errs, fmt.Sprintf("第 %d 个账号的 accountType 不能为空", i+1))
		}
	}
	return errs
}

func defaultConfig() AppConfig {
	return AppConfig{
		Setting: Setting{
			BasicSetting: BasicSetting{
				CompletionTone: 1,
				ColorLog:       1,
				LogOutFileSw:   1,
				LogLevel:       "INFO",
				LogModel:       0,
				WebModel:       0,
			},
			ApiQueSetting: ApiQueSetting{Url: "http://localhost:8083", Protocol: "yatori", Method: "POST", ContentType: "json", AuthType: "none", QuestionField: "content", TypeField: "type", OptionsField: "options", CourseNameField: "courseName", OptionsFormat: "array", AnswerPath: "question.answers", AnswerSplit: "#"},
			AiSetting:     AiSetting{AiType: "TONGYI"},
		},
	}
}

func applyDefaults(cfg *AppConfig) {
	if cfg.Setting.BasicSetting.LogLevel == "" {
		cfg.Setting.BasicSetting.LogLevel = "INFO"
	}
	if cfg.Setting.ApiQueSetting.Url == "" {
		cfg.Setting.ApiQueSetting.Url = "http://localhost:8083"
	}
	if cfg.Setting.ApiQueSetting.Protocol == "" {
		cfg.Setting.ApiQueSetting.Protocol = "yatori"
	}
	if cfg.Setting.ApiQueSetting.Method == "" {
		cfg.Setting.ApiQueSetting.Method = "POST"
	}
	if cfg.Setting.ApiQueSetting.AuthType == "" {
		cfg.Setting.ApiQueSetting.AuthType = "none"
	}
	if cfg.Setting.ApiQueSetting.ContentType == "" {
		cfg.Setting.ApiQueSetting.ContentType = "json"
	}
	if cfg.Setting.ApiQueSetting.QuestionField == "" {
		cfg.Setting.ApiQueSetting.QuestionField = "content"
	}
	if cfg.Setting.ApiQueSetting.TypeField == "" {
		cfg.Setting.ApiQueSetting.TypeField = "type"
	}
	if cfg.Setting.ApiQueSetting.OptionsField == "" {
		cfg.Setting.ApiQueSetting.OptionsField = "options"
	}
	if cfg.Setting.ApiQueSetting.CourseNameField == "" {
		cfg.Setting.ApiQueSetting.CourseNameField = "courseName"
	}
	if cfg.Setting.ApiQueSetting.OptionsFormat == "" {
		cfg.Setting.ApiQueSetting.OptionsFormat = "array"
	}
	if cfg.Setting.ApiQueSetting.TokenParam == "" {
		cfg.Setting.ApiQueSetting.TokenParam = "token"
	}
	if cfg.Setting.ApiQueSetting.AnswerPath == "" {
		cfg.Setting.ApiQueSetting.AnswerPath = "question.answers"
	}
	if cfg.Setting.ApiQueSetting.AnswerSplit == "" {
		cfg.Setting.ApiQueSetting.AnswerSplit = "#"
	}
	cfg.Setting.ApiQueSetting = normalizeQuestionBankSetting(cfg.Setting.ApiQueSetting)
	for i := range cfg.Users {
		NormalizeCoursesCustom(cfg.Users[i].AccountType, &cfg.Users[i].CoursesCustom)
	}
}

func NormalizeCoursesCustom(accountType string, c *CoursesCustom) {
	if c == nil {
		return
	}
	one := 1
	three := 3
	if c.CxNode == nil {
		c.CxNode = &three
	}
	if c.CxChapterTestSw == nil {
		c.CxChapterTestSw = &one
	}
	if c.CxWorkSw == nil {
		c.CxWorkSw = &one
	}
	if c.CxExamSw == nil {
		c.CxExamSw = &one
	}
	if c.IncludeCourses == nil {
		c.IncludeCourses = []string{}
	}
	if c.ExcludeCourses == nil {
		c.ExcludeCourses = []string{}
	}
	if strings.EqualFold(accountType, "XUEXITONG") && c.XxtAutoExamCode == nil {
		c.XxtAutoExamCode = &one
	}
}
