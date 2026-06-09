package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"errors"
	"gorm.io/gorm"
)

func encodePassword(plain string) string {
	return "b64:" + base64.StdEncoding.EncodeToString([]byte(plain))
}

func ensureDB() error {
	if db == nil {
		return InitDB()
	}
	return nil
}

func ListAccounts(mgr *TaskManager) ([]AccountVO, error) {
	if err := ensureDB(); err != nil {
		return nil, err
	}
	var rows []AccountPO
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	supports := map[string]string{}
	for _, p := range PlatformSupportList() {
		supports[p.Code] = p.GUISupport
	}
	out := make([]AccountVO, 0, len(rows))
	for _, r := range rows {
		var cc CoursesCustom
		_ = json.Unmarshal([]byte(r.CoursesCustom), &cc)
		var emails []string
		_ = json.Unmarshal([]byte(r.InformEmails), &emails)
		if emails == nil {
			emails = []string{}
		}
		out = append(out, AccountVO{
			UID:           r.UID,
			AccountType:   r.AccountType,
			URL:           r.URL,
			RemarkName:    r.RemarkName,
			Account:       r.Account,
			IsProxy:       r.IsProxy,
			IsRunning:     mgr.IsRunning(r.UID),
			GUISupport:    supports[r.AccountType],
			CoursesCustom: cc,
			InformEmails:  emails,
		})
	}
	return out, nil
}

func checkDuplicateAccount(req AccountReq) error {
	var count int64
	query := db.Model(&AccountPO{}).
		Where("account_type = ? AND account = ? AND url = ?", req.AccountType, req.Account, req.URL)
	if req.UID != "" {
		query = query.Where("uid <> ?", req.UID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("同一平台、同一入口下已存在该账号。")
	}
	return nil
}

func AddAccount(req AccountReq) error {
	if err := ensureDB(); err != nil {
		return err
	}
	if req.Account == "" || req.AccountType == "" {
		return fmt.Errorf("账号和平台类型不能为空")
	}
	if req.Password == "" {
		return fmt.Errorf("密码不能为空")
	}
	if err := checkDuplicateAccount(req); err != nil {
		return err
	}
	uid, _ := uuid.NewV7()
	ccJSON, _ := json.Marshal(req.CoursesCustom)
	emailsJSON, _ := json.Marshal(req.InformEmails)
	row := AccountPO{
		UID:           uid.String(),
		AccountType:   req.AccountType,
		URL:           req.URL,
		RemarkName:    req.RemarkName,
		Account:       req.Account,
		PasswordEnc:   encodePassword(req.Password),
		IsProxy:       req.IsProxy,
		InformEmails:  string(emailsJSON),
		CoursesCustom: string(ccJSON),
	}
	return db.Create(&row).Error
}

func UpdateAccount(req AccountReq) error {
	if err := ensureDB(); err != nil {
		return err
	}
	if req.UID == "" {
		return fmt.Errorf("UID 不能为空")
	}
	if req.Account == "" || req.AccountType == "" {
		return fmt.Errorf("账号和平台类型不能为空")
	}
	if err := checkDuplicateAccount(req); err != nil {
		return err
	}
	updates := map[string]any{
		"account_type": req.AccountType,
		"url":          req.URL,
		"remark_name":  req.RemarkName,
		"account":      req.Account,
		"is_proxy":     req.IsProxy,
	}
	if req.Password != "" {
		updates["password_enc"] = encodePassword(req.Password)
	}
	if cc, err := json.Marshal(req.CoursesCustom); err == nil {
		updates["courses_custom"] = string(cc)
	}
	if emails, err := json.Marshal(req.InformEmails); err == nil {
		updates["inform_emails"] = string(emails)
	}
	return db.Model(&AccountPO{}).Where("uid = ?", req.UID).Updates(updates).Error
}

func DeleteAccount(uid string) error {
	if err := ensureDB(); err != nil {
		return err
	}
	return db.Where("uid = ?", uid).Delete(&AccountPO{}).Error
}

// GetAccountPO 供 worker 子进程使用（已导出）
func GetAccountPO(uid string) (AccountPO, error) {
	return getAccountPO(uid)
}

// getAccountPO 供 task_manager 内部使用
func getAccountPO(uid string) (AccountPO, error) {
	if err := ensureDB(); err != nil {
		return AccountPO{}, err
	}
	var row AccountPO
	err := db.Where("uid = ?", uid).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AccountPO{}, fmt.Errorf("账号不存在")
	}
	return row, err
}

func GetDashboard(mgr *TaskManager, cfgPath string) (Dashboard, error) {
	_ = ensureDB()
	var total int64
	if db != nil {
		db.Model(&AccountPO{}).Count(&total)
	}
	running := 0
	for _, s := range mgr.Statuses() {
		if s.State == StateRunning {
			running++
		}
	}
	_, cfgErr := LoadConfig(cfgPath)
	logs, _ := TailLog(20)
	return Dashboard{
		TotalAccounts: int(total),
		RunningTasks:  running,
		ConfigPath:    cfgPath,
		ConfigOK:      cfgErr == nil,
		RecentLogs:    logs,
	}, nil
}
