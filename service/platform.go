package service

// PlatformInfo 描述平台对 GUI 的支持程度
type PlatformInfo struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	GUISupport  string `json:"guiSupport"` // "full" | "config-only" | "none"
	Note        string `json:"note"`
}

// PlatformSupportList 诚实列出 GUI 可控程度
// full       = Login + PullCourseList + Start + Stop 均有路径
// config-only = 只能在 UI 编辑配置，单账号启停需人工确认
// none        = 当前版本 web/activity 无实现，UI 标记"暂不支持单账号控制"
func PlatformSupportList() []PlatformInfo {
	return []PlatformInfo{
		{Code: "XUEXITONG", Name: "学习通", GUISupport: "full",
			Note: "web/activity 有 XXTActivity，支持 Login/Start/Stop/PullCourseList"},
		{Code: "YINGHUA", Name: "英华学堂", GUISupport: "full",
			Note: "web/activity 有 YingHuaActivity，支持 Login/Start/Stop；PullCourseList 待确认"},
		{Code: "ENAEA", Name: "学习公社", GUISupport: "config-only",
			Note: "logic 模块存在，web/activity 无实现，UI 仅展示配置"},
		{Code: "CQIE", Name: "重庆工程学院", GUISupport: "config-only",
			Note: "logic 模块存在，web/activity 无实现"},
		{Code: "KETANGX", Name: "码上研训", GUISupport: "config-only",
			Note: "logic 模块存在，web/activity 无实现"},
		{Code: "WELEARN", Name: "WeLearn 随行课堂", GUISupport: "config-only",
			Note: "logic 模块存在，web/activity 无实现"},
		{Code: "ICVE", Name: "智慧职教", GUISupport: "config-only",
			Note: "logic 模块存在，web/activity 无实现"},
		{Code: "QSXT", Name: "青书学堂", GUISupport: "config-only",
			Note: "logic 模块存在，web/activity 无实现"},
		{Code: "HQKJ", Name: "海旗科技", GUISupport: "config-only",
			Note: "logic 模块存在，web/activity 无实现"},
		{Code: "CANGHUI", Name: "仓辉实训", GUISupport: "none",
			Note: "当前版本未见对应 activity 实现"},
	}
}
