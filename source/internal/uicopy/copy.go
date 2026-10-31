// Package uicopy owns all English and Simplified Chinese application copy.
package uicopy

import "github.com/egoist/mygo/i18n"

type message struct{ en, zh string }

func (m message) text() string {
	if i18n.Match([]string{i18n.Locale()}, []string{"zh-Hans"}) != "" {
		return m.zh
	}
	return m.en
}

var messages = map[string]message{
	"Settings":               {"Settings", "设置"},
	"SettingsMenu":           {"Settings…", "设置…"},
	"SettingsWindow":         {"%s Settings", "%s 设置"},
	"PreviewWindow":          {"%s · Preview", "%s · 预览"},
	"PreviewApp":             {"%s Preview", "%s Preview"},
	"PreviewInstanceApp":     {"%s · %s", "%s · %s"},
	"QuitApp":                {"Quit %s", "退出 %s"},
	"SearchSettings":         {"Search settings", "搜索设置"},
	"SettingsCategories":     {"Settings categories", "设置分类"},
	"SearchResults":          {"Search results", "搜索结果"},
	"NoMatchingSettings":     {"No matching settings", "没有匹配的设置"},
	"General":                {"General", "通用"},
	"Tools":                  {"Tools", "工具"},
	"Appearance":             {"Appearance", "外观"},
	"AppearanceSystem":       {"Follow system", "跟随系统"},
	"AppearanceLight":        {"Light", "浅色"},
	"AppearanceDark":         {"Dark", "深色"},
	"OpenAtLogin":            {"Open at login", "登录时打开"},
	"LoginNeedsBundle":       {"Available when packaged as an .app.", "仅在打包为 .app 后可用。"},
	"LoginPreview":           {"Preview does not change login items.", "预览模式不更改系统登录项。"},
	"About":                  {"About", "关于"},
	"Version":                {"Version", "版本"},
	"DevelopmentBuild":       {"Development build", "开发构建"},
	"CopyVersion":            {"Copy version", "复制版本"},
	"VersionCopied":          {"Version copied", "版本已复制"},
	"ProjectPage":            {"Project website", "项目主页"},
	"ProjectOpenFailure":     {"Could not open the project website", "无法打开项目主页"},
	"Dependencies":           {"Dependencies and licenses", "依赖与许可证"},
	"DependenciesDetail":     {"Third-party components included with the app and their original licenses.", "随应用提供的第三方组件及其许可证原文。"},
	"ModifiedDependency":     {"%s (locally modified)", "%s（本地修改）"},
	"HighlightWindow":        {"Pointer highlight", "鼠标位置高亮"},
	"FutureConfigTitle":      {"Settings are from a newer version", "设置来自较新的版本"},
	"FutureConfigDetail":     {"This settings file uses format version %d; this app supports version %d. The original file is preserved. Update the app to save settings. Changes apply to this session only.", "此设置文件使用格式版本 %d，当前应用支持版本 %d。原文件已保留；请升级应用以继续保存设置。本次修改仅本次有效。"},
	"ConfigSchemaError":      {"Unsupported settings format version: %d", "不支持的设置格式版本：%d"},
	"ConfigObjectError":      {"Settings must be a JSON object", "设置必须是 JSON 对象"},
	"SessionOnly":            {"Changes apply to this session only", "修改仅本次有效"},
	"Saving":                 {"Saving…", "正在保存…"},
	"Unsaved":                {"Not saved", "尚未保存"},
	"RecordingShortcut":      {"Recording shortcut", "正在录制快捷键"},
	"PreviewSettings":        {"Preview · Temporary settings", "预览 · 使用临时设置"},
	"SettingsUnavailable":    {"Settings file unavailable", "设置文件不可用"},
	"ReadOnlyDetail":         {"The original file is preserved. Changes will not be saved. Check file permissions and restart the app.", "原文件已保留；本次修改不会保存。请检查文件访问权限后重新启动应用。"},
	"SettingsNotSaved":       {"Settings not saved", "设置未保存"},
	"SaveFailureDetail":      {"Your changes are kept. Retry saving before quitting. %s", "本次修改已保留，退出前请重试保存。 %s"},
	"LoginFailure":           {"Could not change open at login", "无法更改登录启动"},
	"LoginFailureDetail":     {"The switch now reflects the system setting. %s", "开关已恢复为系统实际状态。%s"},
	"Attention":              {"Needs attention", "需要留意"},
	"RetrySave":              {"Retry save", "重试保存"},
	"DiscardAndQuit":         {"Discard and quit", "放弃并退出"},
	"SavePermission":         {"Permission to write the settings file was denied.", "没有写入设置文件的权限。"},
	"SaveDiskFull":           {"The disk is full.", "磁盘空间不足。"},
	"SaveWriteFailure":       {"Could not write the settings file: %s", "无法写入设置文件：%s"},
	"SaveStillPending":       {"Settings are still being saved. Try quitting again shortly.", "设置仍在保存，请稍后再退出。"},
	"ReadSettingsFailure":    {"Could not read settings. Changes will not be saved this session", "读取设置失败，本次运行的修改不会保存"},
	"ToolSettingsFailure":    {"Settings for “%s”: %w", "「%s」的设置：%w"},
	"TrayFailure":            {"Could not create the menu bar icon", "无法创建菜单栏图标"},
	"BackupFailure":          {"The settings file is invalid and could not be backed up. Changes will not be saved this session", "设置文件无效且无法备份，本次运行的修改不会保存"},
	"BackupErrorDetail":      {"%v; backup: %w", "%v；备份：%w"},
	"SettingsRecovered":      {"The invalid settings file was backed up to %s. Default settings are now in use", "设置文件无效，已备份到 %s 并改用默认设置"},
	"ErrorDetail":            {"%s: %s", "%s：%s"},
	"CleanupMismatch":        {"The Dev cleanup request does not match this installation's identity or path", "Dev 清理请求与当前安装身份或路径不符"},
	"CleanupMacOnly":         {"Dev installation cleanup is only supported on macOS", "Dev 安装清理仅支持 macOS"},
	"LoginStatusFailure":     {"Could not read the system login item status", "无法读取系统登录项状态"},
	"UnknownLoginStatus":     {"Unknown login item status: %d", "未知登录项状态：%d"},
	"Shortcuts":              {"Shortcuts", "快捷键"},
	"ShortcutSearchTerm":     {"shortcut", "shortcut"},
	"ShortcutUnset":          {"Not set", "未设置"},
	"ShortcutPrompt":         {"Press a shortcut…", "请按下快捷键…"},
	"ShortcutClear":          {"Clear shortcut", "清除快捷键"},
	"ShortcutReset":          {"Restore default", "恢复默认"},
	"ShortcutLabel":          {"Shortcut for %s", "%s的快捷键"},
	"ShortcutHelp":           {"Esc cancels · Delete clears. Include ⌃, ⌥ or ⌘, or use F1–F12.", "Esc 取消 · Delete 清除；需含 ⌃、⌥ 或 ⌘，或使用 F1–F12。"},
	"ShortcutConflict":       {"Conflicts with the shortcut for “%s”", "与「%s」的快捷键冲突"},
	"ShortcutOccupied":       {"Could not register. The system or another app may be using this shortcut", "无法注册，可能已被系统或其他应用占用"},
	"Options":                {"Options", "选项"},
	"Preview":                {"Preview", "预览"},
	"SoftwareUpdates":        {"Software updates", "软件更新"},
	"CheckUpdates":           {"Check for updates…", "检查更新…"},
	"UpdatesPreview":         {"Preview does not check for release updates.", "预览模式不检查正式版更新。"},
	"UpdatesDev":             {"Dev builds do not receive release updates.", "Dev 版本不参与正式版更新。"},
	"UpdatesUnconfigured":    {"Release updates are not configured for this build.", "此构建未配置正式版自动更新。"},
	"UpdatesUnwritable":      {"Updates cannot be installed here. Move the app to a writable Applications folder.", "当前位置无法安装更新，请移到可写的应用程序目录。"},
	"UpdatesAutomatic":       {"Checks automatically every 24 hours. Review release notes and choose whether to install new versions.", "每 24 小时自动检查，发现新版本后可查看更新说明并选择安装。"},
	"MouseCenter":            {"Center pointer", "鼠标居中"},
	"CenterCurrent":          {"Move to the center of this display", "移到当前屏幕中央"},
	"CenterNext":             {"Move to the center of the next display", "移到下一块屏幕中央"},
	"LocatePointer":          {"Highlight pointer", "高亮鼠标位置"},
	"HighlightOnMove":        {"Highlight after moving", "移动后高亮"},
	"HighlightColor":         {"Highlight color", "高亮颜色"},
	"ColorSystem":            {"System accent", "系统强调色"},
	"ColorRed":               {"Red", "红色"},
	"ColorOrange":            {"Orange", "橙色"},
	"ColorYellow":            {"Yellow", "黄色"},
	"ColorGreen":             {"Green", "绿色"},
	"ColorPink":              {"Pink", "粉色"},
	"Language":               {"Language", "语言"},
	"LanguageSystem":         {"Follow system", "跟随系统"},
	"LanguageEnglish":        {"English", "English"},
	"LanguageChinese":        {"简体中文", "简体中文"},
	"UpdateRedirectHTTPS":    {"update redirect must use HTTPS", "更新重定向必须使用 HTTPS"},
	"UpdateRedirectLimit":    {"too many update redirects", "更新重定向次数过多"},
	"UpdateDownloadHTTPS":    {"update download must use HTTPS", "更新下载必须使用 HTTPS"},
	"UpdateServerHTTP":       {"update server returned HTTP %d", "更新服务器返回 HTTP %d"},
	"UpdateStableOnly":       {"this update source supports stable releases only", "此更新源仅支持正式版本"},
	"UpdateManifestLimit":    {"update manifest exceeds size limit", "更新清单超出大小限制"},
	"UpdateStablePrerelease": {"the stable update feed cannot offer a prerelease", "正式版更新源不能提供预发布版本"},
	"UpdateExpired":          {"update candidate expired; check for updates again", "更新候选已失效，请重新检查更新"},
	"UpdateDownloadLimit":    {"update archive exceeds declared size", "更新 DMG 超出声明的大小"},
	"UpdateValidation":       {"update archive validation: %w", "更新 DMG 校验失败：%w"},
	"UpdateVersionInvalid":   {"invalid semantic version: %q", "无效的语义版本号：%q"},
	"UpdateSourceMismatch":   {"manifest release source mismatch", "更新清单的发布来源不匹配"},
	"UpdateSizeInvalid":      {"invalid or oversized update archive", "更新 DMG 无效或超出大小限制"},
	"UpdateDeltaUnsupported": {"delta updates are not supported", "不支持差分更新"},
	"UpdateSignatureInvalid": {"invalid update signature encoding or size", "更新签名的编码或长度无效"},
	"UpdateNotNewer":         {"update version must be newer than the installed version", "更新版本必须高于已安装版本"},
	"UpdateManifestInvalid":  {"invalid or oversized manifest", "更新清单无效或超出大小限制"},
	"UpdateVersionMismatch":  {"manifest version mismatch", "更新清单的版本不匹配"},
	"UpdateFileMismatch":     {"archive size or file type mismatch", "更新 DMG 的大小或文件类型不匹配"},
	"UpdateCommitMismatch":   {"archive release commit mismatch", "更新 DMG 的源码提交不匹配"},
	"UpdatePublicKeyInvalid": {"invalid pinned update public key", "内置更新公钥无效"},
	"UpdateLengthMismatch":   {"archive size mismatch", "更新 DMG 大小不匹配"},
	"UpdateLengthChanged":    {"archive size changed during verification", "更新 DMG 的大小在校验期间发生变化"},
	"UpdateUnsigned":         {"archive is not signed by the pinned update key", "更新 DMG 未由内置公钥对应的密钥签名"},
	"UpdateMacOnly":          {"DMG inspection requires macOS", "DMG 检查仅支持 macOS"},
	"UpdateMountFailure":     {"cannot mount update DMG: %w: %s", "无法挂载更新 DMG：%w：%s"},
	"UpdateUnmountFailure":   {"cannot unmount update DMG at %s: %w: %s", "无法卸载位于 %s 的更新 DMG：%w：%s"},
	"UpdateStageFailure":     {"cannot stage update bundle: %w: %s", "无法暂存更新应用：%w：%s"},
	"UpdateUnsafeEntry":      {"unsafe or duplicate update bundle entry: %q", "更新应用中存在不安全或重复的文件路径：%q"},
	"UpdateExpandedLimit":    {"expanded update exceeds limit", "更新展开后超出大小限制"},
	"UpdateMetadataInvalid":  {"invalid or oversized build metadata", "构建元数据无效或超出大小限制"},
	"UpdateIdentityMismatch": {"archive release identity or executable mismatch", "更新应用的发布身份或可执行文件不匹配"},
}

func Settings() string            { return messages["Settings"].text() }
func SettingsMenu() string        { return messages["SettingsMenu"].text() }
func SettingsWindow() string      { return messages["SettingsWindow"].text() }
func PreviewWindow() string       { return messages["PreviewWindow"].text() }
func PreviewApp() string          { return messages["PreviewApp"].text() }
func PreviewInstanceApp() string  { return messages["PreviewInstanceApp"].text() }
func QuitApp() string             { return messages["QuitApp"].text() }
func SearchSettings() string      { return messages["SearchSettings"].text() }
func SettingsCategories() string  { return messages["SettingsCategories"].text() }
func SearchResults() string       { return messages["SearchResults"].text() }
func NoMatchingSettings() string  { return messages["NoMatchingSettings"].text() }
func General() string             { return messages["General"].text() }
func Tools() string               { return messages["Tools"].text() }
func Appearance() string          { return messages["Appearance"].text() }
func AppearanceSystem() string    { return messages["AppearanceSystem"].text() }
func AppearanceLight() string     { return messages["AppearanceLight"].text() }
func AppearanceDark() string      { return messages["AppearanceDark"].text() }
func OpenAtLogin() string         { return messages["OpenAtLogin"].text() }
func LoginNeedsBundle() string    { return messages["LoginNeedsBundle"].text() }
func LoginPreview() string        { return messages["LoginPreview"].text() }
func About() string               { return messages["About"].text() }
func Version() string             { return messages["Version"].text() }
func DevelopmentBuild() string    { return messages["DevelopmentBuild"].text() }
func CopyVersion() string         { return messages["CopyVersion"].text() }
func VersionCopied() string       { return messages["VersionCopied"].text() }
func ProjectPage() string         { return messages["ProjectPage"].text() }
func ProjectOpenFailure() string  { return messages["ProjectOpenFailure"].text() }
func Dependencies() string        { return messages["Dependencies"].text() }
func DependenciesDetail() string  { return messages["DependenciesDetail"].text() }
func ModifiedDependency() string  { return messages["ModifiedDependency"].text() }
func HighlightWindow() string     { return messages["HighlightWindow"].text() }
func FutureConfigTitle() string   { return messages["FutureConfigTitle"].text() }
func FutureConfigDetail() string  { return messages["FutureConfigDetail"].text() }
func ConfigSchemaError() string   { return messages["ConfigSchemaError"].text() }
func ConfigObjectError() string   { return messages["ConfigObjectError"].text() }
func SessionOnly() string         { return messages["SessionOnly"].text() }
func Saving() string              { return messages["Saving"].text() }
func Unsaved() string             { return messages["Unsaved"].text() }
func RecordingShortcut() string   { return messages["RecordingShortcut"].text() }
func PreviewSettings() string     { return messages["PreviewSettings"].text() }
func SettingsUnavailable() string { return messages["SettingsUnavailable"].text() }
func ReadOnlyDetail() string      { return messages["ReadOnlyDetail"].text() }
func SettingsNotSaved() string    { return messages["SettingsNotSaved"].text() }
func SaveFailureDetail() string   { return messages["SaveFailureDetail"].text() }
func LoginFailure() string        { return messages["LoginFailure"].text() }
func LoginFailureDetail() string  { return messages["LoginFailureDetail"].text() }
func Attention() string           { return messages["Attention"].text() }
func RetrySave() string           { return messages["RetrySave"].text() }
func DiscardAndQuit() string      { return messages["DiscardAndQuit"].text() }
func SavePermission() string      { return messages["SavePermission"].text() }
func SaveDiskFull() string        { return messages["SaveDiskFull"].text() }
func SaveWriteFailure() string    { return messages["SaveWriteFailure"].text() }
func SaveStillPending() string    { return messages["SaveStillPending"].text() }
func ReadSettingsFailure() string { return messages["ReadSettingsFailure"].text() }
func ToolSettingsFailure() string { return messages["ToolSettingsFailure"].text() }
func TrayFailure() string         { return messages["TrayFailure"].text() }
func BackupFailure() string       { return messages["BackupFailure"].text() }
func BackupErrorDetail() string   { return messages["BackupErrorDetail"].text() }
func SettingsRecovered() string   { return messages["SettingsRecovered"].text() }
func ErrorDetail() string         { return messages["ErrorDetail"].text() }
func CleanupMismatch() string     { return messages["CleanupMismatch"].text() }
func CleanupMacOnly() string      { return messages["CleanupMacOnly"].text() }
func LoginStatusFailure() string  { return messages["LoginStatusFailure"].text() }
func UnknownLoginStatus() string  { return messages["UnknownLoginStatus"].text() }
func Shortcuts() string           { return messages["Shortcuts"].text() }
func ShortcutSearchTerm() string  { return messages["ShortcutSearchTerm"].text() }
func ShortcutUnset() string       { return messages["ShortcutUnset"].text() }
func ShortcutPrompt() string      { return messages["ShortcutPrompt"].text() }
func ShortcutClear() string       { return messages["ShortcutClear"].text() }
func ShortcutReset() string       { return messages["ShortcutReset"].text() }
func ShortcutLabel() string       { return messages["ShortcutLabel"].text() }
func ShortcutHelp() string        { return messages["ShortcutHelp"].text() }
func ShortcutConflict() string    { return messages["ShortcutConflict"].text() }
func ShortcutOccupied() string    { return messages["ShortcutOccupied"].text() }
func Options() string             { return messages["Options"].text() }
func Preview() string             { return messages["Preview"].text() }
func SoftwareUpdates() string     { return messages["SoftwareUpdates"].text() }
func CheckUpdates() string        { return messages["CheckUpdates"].text() }
func UpdatesPreview() string      { return messages["UpdatesPreview"].text() }
func UpdatesDev() string          { return messages["UpdatesDev"].text() }
func UpdatesUnconfigured() string { return messages["UpdatesUnconfigured"].text() }
func UpdatesUnwritable() string   { return messages["UpdatesUnwritable"].text() }
func UpdatesAutomatic() string    { return messages["UpdatesAutomatic"].text() }
func MouseCenter() string         { return messages["MouseCenter"].text() }
func CenterCurrent() string       { return messages["CenterCurrent"].text() }
func CenterNext() string          { return messages["CenterNext"].text() }
func LocatePointer() string       { return messages["LocatePointer"].text() }
func HighlightOnMove() string     { return messages["HighlightOnMove"].text() }
func HighlightColor() string      { return messages["HighlightColor"].text() }
func ColorSystem() string         { return messages["ColorSystem"].text() }
func ColorRed() string            { return messages["ColorRed"].text() }
func ColorOrange() string         { return messages["ColorOrange"].text() }
func ColorYellow() string         { return messages["ColorYellow"].text() }
func ColorGreen() string          { return messages["ColorGreen"].text() }
func ColorPink() string           { return messages["ColorPink"].text() }
func Language() string            { return messages["Language"].text() }
func LanguageSystem() string      { return messages["LanguageSystem"].text() }
func LanguageEnglish() string     { return messages["LanguageEnglish"].text() }
func LanguageChinese() string     { return messages["LanguageChinese"].text() }

func UpdateRedirectHTTPS() string    { return messages["UpdateRedirectHTTPS"].text() }
func UpdateRedirectLimit() string    { return messages["UpdateRedirectLimit"].text() }
func UpdateDownloadHTTPS() string    { return messages["UpdateDownloadHTTPS"].text() }
func UpdateServerHTTP() string       { return messages["UpdateServerHTTP"].text() }
func UpdateStableOnly() string       { return messages["UpdateStableOnly"].text() }
func UpdateManifestLimit() string    { return messages["UpdateManifestLimit"].text() }
func UpdateStablePrerelease() string { return messages["UpdateStablePrerelease"].text() }
func UpdateExpired() string          { return messages["UpdateExpired"].text() }
func UpdateDownloadLimit() string    { return messages["UpdateDownloadLimit"].text() }
func UpdateValidation() string       { return messages["UpdateValidation"].text() }
func UpdateVersionInvalid() string   { return messages["UpdateVersionInvalid"].text() }
func UpdateSourceMismatch() string   { return messages["UpdateSourceMismatch"].text() }
func UpdateSizeInvalid() string      { return messages["UpdateSizeInvalid"].text() }
func UpdateDeltaUnsupported() string { return messages["UpdateDeltaUnsupported"].text() }
func UpdateSignatureInvalid() string { return messages["UpdateSignatureInvalid"].text() }
func UpdateNotNewer() string         { return messages["UpdateNotNewer"].text() }
func UpdateManifestInvalid() string  { return messages["UpdateManifestInvalid"].text() }
func UpdateVersionMismatch() string  { return messages["UpdateVersionMismatch"].text() }
func UpdateFileMismatch() string     { return messages["UpdateFileMismatch"].text() }
func UpdateCommitMismatch() string   { return messages["UpdateCommitMismatch"].text() }
func UpdatePublicKeyInvalid() string { return messages["UpdatePublicKeyInvalid"].text() }
func UpdateLengthMismatch() string   { return messages["UpdateLengthMismatch"].text() }
func UpdateLengthChanged() string    { return messages["UpdateLengthChanged"].text() }
func UpdateUnsigned() string         { return messages["UpdateUnsigned"].text() }
func UpdateMacOnly() string          { return messages["UpdateMacOnly"].text() }
func UpdateMountFailure() string     { return messages["UpdateMountFailure"].text() }
func UpdateUnmountFailure() string   { return messages["UpdateUnmountFailure"].text() }
func UpdateStageFailure() string     { return messages["UpdateStageFailure"].text() }
func UpdateUnsafeEntry() string      { return messages["UpdateUnsafeEntry"].text() }
func UpdateExpandedLimit() string    { return messages["UpdateExpandedLimit"].text() }
func UpdateMetadataInvalid() string  { return messages["UpdateMetadataInvalid"].text() }
func UpdateIdentityMismatch() string { return messages["UpdateIdentityMismatch"].text() }
