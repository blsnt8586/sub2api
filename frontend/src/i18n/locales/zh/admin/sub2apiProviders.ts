export default {
  sub2apiProviders: {
    title: '上游管理',
    description: '管理 Sub2API 上游实例及账号倍率优化',

    // 列表操作
    searchProviders: '搜索上游名称…',
    allStatus: '全部状态',
    refresh: '刷新',
    createProvider: '新增上游',

    // 路径探测
    pathsReady: '路径已就绪',
    pathsNotDetected: '路径未探测',
    pathsNotDetectedHint: '路径尚未探测，账号的实际路径可能偏差较大',

    // 状态标签
    statusLabels: {
      active: '已启用',
      inactive: '已停用',
    },

    // 同步状态
    syncStatus: {
      never: '未同步',
      success: '成功',
      failed: '失败',
      pending: '等待中',
      running: '运行中',
    },

    // 空状态
    noProviders: '暂无上游实例',
    noProvidersDesc: '点击「新增上游」添加第一个 Sub2API 上游实例',

    // 关联账号面板
    viewAccounts: '查看关联账号',
    linkedAccounts: '关联账号',
    noLinkedAccounts: '暂无关联账号',
    linkAccount: '关联账号',
    linkAccountTitle: '关联账号',
    linkAccountDesc: '选择要关联到此上游的账号',
    selectAccount: '选择账号',
    selectAccountPlaceholder: '搜索或选择账号…',
    linking: '关联中…',
    linked: '关联成功',
    linkFailed: '关联失败',
    unlinkAccount: '解除关联',
    unlinked: '已解除关联',
    unlinkFailed: '解除关联失败',
    refreshRemote: '同步远端账号',

    // 优化
    optimizeAll: '批量优化',
    optimizeAllTitle: '批量优化所有账号',
    optimizeAccount: '立即优化',
    optimizeFailed: '优化失败',
    alreadyOptimal: '已是最优，无需调整',
    optimizeStatOptimized: '已优化',
    optimizeStatSkipped: '已跳过',
    optimizeStatFailed: '失败',

    // 表格列
    colAccountName: '账号名称',
    colPlatform: '平台',
    colCurrentGroup: '当前分组',
    colMultiplier: '当前倍率',
    colActions: '操作',

    // 参与调度
    joinSchedule: '参与调度',
    joinScheduleHint: '开启后此账号会被纳入定时优化任务',
    joinScheduleOn: '已参与调度',
    joinScheduleOff: '未参与调度',

    // 倍率限制
    maxMultiplier: '上限倍率',
    maxMultiplierHint: '账号倍率优化的最大值（留空不限）',
    minMultiplier: '下限倍率',
    minMultiplierHint: '账号倍率优化的最小值（留空不限）',

    // 测试模型
    testModel: '测试模型',
    testModelHint: '优化时用于连通性探测的模型，留空使用默认',
    defaultModel: '默认模型',

    // 连通性测试
    testConnection: '测试连通',
    connectionSuccess: '连通测试成功',
    connectionFailed: '连通测试失败',

    // 探测路径
    detectPaths: '探测路径',
    pathsDetectFailed: '路径探测失败',

    // 表单字段
    form: {
      name: '上游名称',
      namePlaceholder: '给这个上游起个名字',
      baseUrl: 'Base URL',
      baseUrlPlaceholder: 'https://your-sub2api.example.com',
      email: '登录邮箱',
      emailPlaceholder: 'admin@example.com',
      password: '登录密码',
      passwordPlaceholder: '留空不修改',
      providerType: '上游类型',
      providerTypeHint: '目前仅支持 sub2api 协议',
      status: '状态',
      notes: '备注',
      notesPlaceholder: '可选备注信息',
    },

    // 编辑 / 删除
    editProvider: '编辑',
    deleteProvider: '删除',
    enableProvider: '启用',
    disableProvider: '停用',

    // 反馈消息
    createSuccess: '上游实例已创建',
    updateSuccess: '已更新',
    deleteSuccess: '已删除',
    deleteFailed: '删除失败',
    saveFailed: '保存失败',
    settingsSaved: '设置已保存',
    settingsSaveFailed: '设置保存失败',
    loadFailed: '加载失败',

    // 定时优化调度
    scheduleOptimize: '定时优化',
    scheduleOptimizeTitle: '配置定时优化',
    scheduleOptimizeDesc: '设置 cron 表达式，系统将按计划自动对所有参与调度的账号进行倍率优化',
    cronExpr: 'Cron 表达式',
    cronExprPlaceholder: '0 2 * * * (每天凌晨 2 点)',
    cronPresets: {
      hourly: '每小时',
      daily2am: '每天凌晨 2 点',
      daily6am: '每天早上 6 点',
      every6h: '每 6 小时',
      weekly: '每周一凌晨 2 点',
    },
    scheduleEnabled: '启用定时优化',
    scheduleSaved: '调度配置已保存',
    scheduleSaveFailed: '调度配置保存失败',
    scheduleDeleted: '调度已删除',
    deleteSchedule: '删除调度',
    deleteScheduleConfirm: '确定要删除此定时调度吗？',
    runNow: '立即执行',
    runNowSuccess: '已触发立即执行',
    runNowFailed: '触发执行失败',
    runningHint: '任务正在运行中，请稍后刷新查看结果',
    lastRunAt: '上次执行',
    nextRunAt: '下次执行',
    neverRun: '从未执行',
    recentLogs: '近期日志',
    noLogs: '暂无日志',
  },
}
