window.H2_DESIGN_CANVAS = {
 title: "odds · 原生设置设计",
 boards: [
  {file:"about-details.html",title:"11 / 关于 · 依赖与许可证",x:840,y:1160,w:760,h:560},
  {file:"future-config.html",title:"12 / 较新版设置 · 只读保护",x:1680,y:1160,w:760,h:560},
  {file:"about-details.html",title:"13 / 关于 · 最小窗口",x:4200,y:1160,w:480,h:420},
  {file:"general-dev.html",title:"10 / Dev · 独立身份与更新禁用",x:0,y:1160,w:760,h:560},
  {file:"general.html",title:"01 / 通用 · 外观、后台与关于",x:0,y:120,w:760,h:560},
  {file:"mouse-center.html",title:"02 / 鼠标居中 · 正常",x:840,y:120,w:760,h:560},
  {file:"recording.html",title:"03 / 快捷键 · 录制中",x:1680,y:120,w:760,h:560},
  {file:"save-failed.html",title:"04 / 保存失败 · 保留输入与重试",x:2520,y:120,w:760,h:560},
  {file:"quit-blocked.html",title:"05 / 退出受阻 · 重试或明确放弃",x:2520,y:1160,w:760,h:560},
  {file:"save-retrying.html",title:"09 / 重试保存 · 保留上次错误",x:3360,y:1160,w:760,h:560},
  {file:"read-only.html",title:"06 / 设置不可读 · 保留原件",x:3360,y:120,w:760,h:560},
  {file:"mouse-center-dark.html",title:"07 / 鼠标居中 · 深色",x:4200,y:120,w:760,h:560},
  {file:"mouse-center.html",title:"08 / 最小窗口 · 480 × 420",x:5040,y:120,w:480,h:420}
 ],
 notes: [
  {text:"沿用 Onevra 的暖白、炭灰与陶土色。画板表达内容与状态；原生控件以 MyGo / Onevra 实际渲染为准。正式版复用 MyGo 原生更新窗口展示检查、说明、下载、可重启与错误状态；Dev 不检查正式 Feed。默认窗口保持 760 × 560，内容可滚动。",x:0,y:0,w:760},
  {text:"录制时暂停全局快捷键；取消、失焦、隐藏或切换分区后恢复。空快捷键明确显示未设置。",x:1680,y:0,w:760},
  {text:"保存失败保留本次修改。全局提示始终可见，不依赖当前分区与搜索；重试成功后清除错误。退出受阻时另提供明确的放弃并退出动作。",x:2520,y:0,w:760},
  {text:"窗口小于 620 时使用分类选择器，控件排在说明下方，正文可滚动。HTML 预览不代替原生窗口与菜单栏验证。",x:5040,y:0,w:480}
 ]
};
