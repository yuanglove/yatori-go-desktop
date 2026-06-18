package service

type PlatformInfo struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	GUISupport string `json:"guiSupport"` // "full" | "config-only"
	Note       string `json:"note"`
}

// full = worker 子进程里 log.Fatal/os.Exit 只杀子进程，直接复用原 logic 包
func PlatformSupportList() []PlatformInfo {
	return []PlatformInfo{
		{Code: "XUEXITONG", Name: "学习通", GUISupport: "full",
			Note: "支持视频/文档/章节测验/作业/考试，VideoModel 1/2/3"},
		{Code: "YINGHUA", Name: "英华学堂", GUISupport: "full",
			Note: "支持视频 VideoModel 1/2/3、作业、考试"},
		{Code: "CANGHUI", Name: "仓辉实训", GUISupport: "full",
			Note: "按英华兼容路径运行；如学校是英华套壳，也可直接使用 YINGHUA"},
		{Code: "ENAEA", Name: "学习公社", GUISupport: "full",
			Note: "支持视频 VideoModel 1/2，worker 子进程运行"},
		{Code: "QSXT", Name: "青书学堂", GUISupport: "full",
			Note: "支持视频/资料/作业，worker 子进程运行"},
		{Code: "CQIE", Name: "重庆智慧教育", GUISupport: "full",
			Note: "支持视频 VideoModel 1/2，worker 子进程运行"},
		{Code: "WELEARN", Name: "WeLearn 随行课堂", GUISupport: "full",
			Note: "支持视频 VideoModel 1/2，worker 子进程运行"},
		{Code: "ICVE", Name: "智慧职教", GUISupport: "full",
			Note: "仅支持 Cookie 登录，VideoModel 1，worker 子进程运行"},
		{Code: "HQKJ", Name: "海旗科技", GUISupport: "full",
			Note: "支持视频 VideoModel 1/2，worker 子进程运行"},
		{Code: "KETANGX", Name: "码上研训", GUISupport: "full",
			Note: "支持视频 VideoModel 1，worker 子进程运行"},
	}
}
