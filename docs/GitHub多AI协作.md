# GitHub与103的多AI协作

代码远端：https://github.com/apllozhang/OmniRackNg2。运行环境入口：/home/tina/OmniRackWorkspace/START-HERE.md。

## 任务流程

1. Issue写明目标、验收条件、操作者、依赖和允许修改文件；负责人/整合者分配，AI不能假装另一个AI已停止工作。
2. 103执行omni task-new TASK --actor AI_NAME --goal '目标'，使用work/TASK；外部AI可用独立clone/worktree，保持相同分支约定，不能共用一个目录。
3. 一个AI负责一个PR。发生文件交叉时先协调；禁止整文件覆盖别人的修改。GitHub PR/commit是代码事实，103 HANDOFF是任务及运行事实，两者都需检查。
4. PR运行backend、frontend、integration检查；写清没有执行的浏览器/内网/真实外部依赖测试。复核者独立看代码和原始证据。
5. 负责人/整合者合并，禁止强推main/改发布标签；同一GitHub账号下多个AI无法提供真正独立的GitHub审批，需其他授权评审账号或负责人审阅。
6. 合并后103拉取并保持main干净；实际部署身份以OmniRackReplica-current/RELEASE.json为准，不把GitHub HEAD当线上版本。
7. 只有指定发布者使用103受控工具发布15174。15173/19173对照保持只读。CI不自动发布，禁止PR代码在NAS上以高权限self-hosted runner执行。
8. 换手/结束前更新Issue/PR与103交接：实际HEAD、脏文件、验证、发布状态、风险、下一步，禁止只留聊天总结。

## 远端与公开边界

103内部origin保留本地裸仓库，用github命名GitHub远端；GitHub主线与内部main整合时只允许快进或评审过的合并，不强行覆盖。

首次只推main和确认过的发布标签；不使用--mirror，不上传refs/stash、内部任务草稿、备份、env、数据库、原始安装包或含真实数据的证据。机器路径和代码文档可公开；私有证据仅保留103。

CI固定官方Actions提交、只读contents权限，使用pull_request而非执行不可信代码的pull_request_target。不让提交者修改流程后绕过应有审批。main要求PR、至少一位独立审批、backend/frontend/integration通过、解决评审讨论；规则适用于管理员。初次导入之后禁止直接提交main。

## 初始基线

上线版本0.2.0-preview.16-webui.1，部署源码提交b500806。GitHub接入与协作配置是后续提交，不改变运行实例；从RELEASE.json复核最新状态。既有生产安全R01–R12及WebUI完整表格验收事项继续有效。
