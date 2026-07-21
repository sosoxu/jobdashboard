<template>
  <div class="page">
    <div class="card mb-16">
      <div class="flex-between">
        <div>
          <el-button text :icon="Back" @click="$router.push('/jobs')">返回</el-button>
          <span class="log-title">作业日志：{{ jobDesc || jobName }}</span>
        </div>
        <div class="muted">项目 {{ project || '-' }} / 工区 {{ survey || '-' }}</div>
      </div>
    </div>

    <div class="card">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <el-tab-pane label="list 日志" name="list" />
        <el-tab-pane label="LOG 日志" name="log" />
        <el-tab-pane label="AI 日志分析" name="ai" />
      </el-tabs>

      <template v-if="activeTab === 'ai'">
        <div v-if="!analyze" class="muted">点击"分析"按钮开始诊断…</div>
        <div v-else>
          <el-alert :title="analyze.summary" type="info" :closable="false" class="mb-16" />
          <el-row :gutter="16">
            <el-col :span="12">
              <h4>模块错误分布</h4>
              <div v-if="analyze.moduleErrors.length === 0" class="muted">无错误</div>
              <div v-for="m in analyze.moduleErrors" :key="m.module" class="module-row">
                <span>{{ m.module }}</span>
                <el-progress
                  :percentage="errPct(m.count)"
                  :stroke-width="14"
                  :format="() => String(m.count)"
                />
              </div>
            </el-col>
            <el-col :span="12">
              <h4>诊断建议</h4>
              <div v-if="analyze.suggestions.length === 0" class="muted">无建议</div>
              <ul class="suggestions">
                <li v-for="s in analyze.suggestions" :key="s.code">
                  <el-tag :type="sevType(s.severity)" size="small">{{ s.code }}</el-tag>
                  <span class="sugg-desc">{{ s.desc }}</span>
                </li>
              </ul>
            </el-col>
          </el-row>
          <div class="mt-16">
            <el-button type="primary" :loading="analyzing" @click="runAnalyze">重新分析</el-button>
          </div>
        </div>
      </template>

      <template v-else>
        <div class="flex-between mb-16 flex-wrap">
          <div class="flex gap-8 flex-wrap items-center">
            <el-input
              v-model="keyword"
              placeholder="关键字（在原始文本中搜索）"
              clearable
              size="small"
              style="width: 240px"
              @keyup.enter="onFilterChange"
              @clear="onFilterChange"
            />
            <el-button size="small" type="primary" @click="onFilterChange">搜索</el-button>
            <el-button
              v-if="log?.filtered"
              size="small"
              @click="clearKeyword"
            >清除过滤</el-button>
            <el-select v-model="pageSize" size="small" style="width: 110px" @change="onPageSizeChange">
              <el-option :value="100" label="100 行/页" />
              <el-option :value="200" label="200 行/页" />
              <el-option :value="500" label="500 行/页" />
              <el-option :value="1000" label="1000 行/页" />
            </el-select>
            <!-- 段落快速跳转：仅 list 类型且有段落时显示 -->
            <template v-if="activeTab === 'list' && sections.length > 0">
              <el-divider direction="vertical" />
              <span class="muted">段落定位：</span>
              <el-select
                v-model="selectedSectionLine"
                placeholder="跳转到段落..."
                size="small"
                style="width: 280px"
                filterable
                @change="jumpToSection"
              >
                <el-option
                  v-for="s in sections"
                  :key="s.lineNo"
                  :value="s.lineNo"
                  :label="`L${s.lineNo} · ${s.name}`"
                />
              </el-select>
            </template>
          </div>
          <span v-if="log" class="muted log-meta">
            共 {{ totalLabel }} 行 · 第 {{ log.page }}/{{ totalPagesLabel }} 页 ·
            {{ log.filtered ? '已过滤' : '未过滤' }}{{ log.cached ? ' · 缓存命中' : '' }}{{ log.truncated ? ' · 已截断' : '' }}
            · {{ log.path }}
          </span>
        </div>
        <div v-loading="loading" class="log-box" :style="{ maxHeight: logBoxHeight }">
          <div v-if="log && log.lines.length === 0" class="muted log-empty">无匹配日志</div>
          <div
            v-for="l in log?.lines ?? []"
            :key="l.lineNo"
            class="log-line"
            :class="{ 'section-mark': isSectionStart(l.lineNo) }"
            :data-line="l.lineNo"
          >
            <span class="lineno">{{ l.lineNo }}</span>
            <span class="linetext">{{ l.text || ' ' }}</span>
          </div>
        </div>
        <div v-if="log" class="pagination-bar">
          <el-pagination
            v-model:current-page="page"
            :page-size="pageSize"
            :total="paginationTotal"
            layout="prev, pager, next, jumper, total"
            background
            @current-change="loadLogs"
          />
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { Back } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getLogs, analyzeLogs } from '@/api/log'
import type { LogResult, AnalyzeResult, LogSection } from '@/api/types'

const route = useRoute()
const jobName = computed(() => String(route.params.jobName || ''))
const jobDesc = computed(() => String(route.query.jobDesc || ''))

const activeTab = ref<'list' | 'log' | 'ai'>('list')
const keyword = ref('')
const page = ref(1)
const pageSize = ref(200)
const log = ref<LogResult | null>(null)
const loading = ref(false)

const analyze = ref<AnalyzeResult | null>(null)
const analyzing = ref(false)

// 段落快速跳转
const selectedSectionLine = ref<number | null>(null)

// 段落列表（统一从 log.sections 取，便于模板使用）
const sections = computed<LogSection[]>(() => log.value?.sections ?? [])

// 分页器 total：未知时返回当前页+1（允许向后翻页）
const paginationTotal = computed(() => {
  if (!log.value) return 0
  if (log.value.total >= 0) return log.value.total
  return page.value * pageSize.value + (log.value.lines.length >= pageSize.value ? 1 : 0)
})

const totalLabel = computed(() => {
  if (!log.value) return '0'
  if (log.value.total >= 0) return String(log.value.total)
  return '未知'
})

const totalPagesLabel = computed(() => {
  if (!log.value) return '?'
  if (log.value.total >= 0) {
    return String(Math.max(1, Math.ceil(log.value.total / pageSize.value)))
  }
  return '?'
})

const logBoxHeight = computed(() => '60vh')

async function loadLogs() {
  if (activeTab.value === 'ai') return
  loading.value = true
  try {
    log.value = await getLogs(jobName.value, {
      type: activeTab.value,
      keyword: keyword.value,
      page: page.value,
      pageSize: pageSize.value,
    })
  } catch (e: any) {
    ElMessage.error(e.message || '读取日志失败')
    log.value = null
  } finally {
    loading.value = false
  }
}

async function runAnalyze() {
  analyzing.value = true
  try {
    analyze.value = await analyzeLogs(jobName.value, {
      type: 'log',
      keyword: keyword.value,
      page: 1,
      pageSize: 2000,
    })
  } catch (e: any) {
    ElMessage.error(e.message || '分析失败')
  } finally {
    analyzing.value = false
  }
}

function onTabChange(name: string | number) {
  const tab = String(name)
  selectedSectionLine.value = null
  if (tab === 'ai') {
    if (!analyze.value) void runAnalyze()
  } else {
    page.value = 1
    void loadLogs()
  }
}

function onFilterChange() {
  page.value = 1
  void loadLogs()
}

function clearKeyword() {
  keyword.value = ''
  page.value = 1
  void loadLogs()
}

function onPageSizeChange() {
  page.value = 1
  void loadLogs()
}

// 跳转到指定段落所在页
function jumpToSection(lineNo: number | null) {
  if (lineNo == null) return
  // 过滤模式下不切换，避免段落行号与过滤后行号错位
  if (keyword.value) {
    ElMessage.info('请先清除关键字过滤，再使用段落定位')
    selectedSectionLine.value = null
    return
  }
  const targetPage = Math.floor((lineNo - 1) / pageSize.value) + 1
  if (targetPage !== page.value) {
    page.value = targetPage
    void loadLogs().then(() => scrollToLine(lineNo))
  } else {
    scrollToLine(lineNo)
  }
}

function scrollToLine(lineNo: number) {
  // 等待 DOM 更新后滚动
  setTimeout(() => {
    const el = document.querySelector(`.log-line[data-line="${lineNo}"]`) as HTMLElement | null
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }, 100)
}

// 判断当前行是否是段落起始行（用于高亮）
function isSectionStart(lineNo: number): boolean {
  if (!sections.value || sections.value.length === 0) return false
  return sections.value.some(s => s.lineNo === lineNo)
}

function errPct(count: number): number {
  const max = Math.max(1, ...(analyze.value?.moduleErrors.map((m) => m.count) ?? [1]))
  return Math.round((count / max) * 100)
}

function sevType(sev: string): 'danger' | 'warning' | 'info' {
  if (sev === 'high') return 'danger'
  if (sev === 'medium') return 'warning'
  return 'info'
}

onMounted(() => {
  void loadLogs()
})
</script>

<style scoped>
.log-title {
  font-weight: 600;
  margin-left: 8px;
}
.log-meta {
  font-size: 12px;
  max-width: 60%;
  text-align: right;
  word-break: break-all;
}
.log-box {
  overflow: auto;
  background: #1e1e1e;
  border-radius: 6px;
  padding: 8px;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
}
.log-line {
  display: flex;
  gap: 12px;
  padding: 1px 4px;
  color: #d4d4d4;
  white-space: pre-wrap;
  word-break: break-all;
}
.log-line.section-mark {
  background: rgba(64, 158, 255, 0.12);
  border-left: 3px solid #409eff;
  padding-left: 4px;
  margin-top: 4px;
}
.lineno {
  flex: 0 0 60px;
  color: #6e7681;
  text-align: right;
  user-select: none;
}
.linetext {
  flex: 1;
}
.log-empty {
  color: #6e7681;
  padding: 32px 0;
  text-align: center;
}
.module-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}
.module-row > span:first-child {
  flex: 0 0 70px;
}
.suggestions {
  list-style: none;
  padding: 0;
  margin: 0;
}
.suggestions li {
  margin-bottom: 10px;
  display: flex;
  gap: 8px;
  align-items: flex-start;
}
.sugg-desc {
  font-size: 13px;
}
h4 {
  margin: 0 0 12px;
}
.pagination-bar {
  margin-top: 12px;
  display: flex;
  justify-content: center;
}
.flex-wrap {
  flex-wrap: wrap;
}
.items-center {
  align-items: center;
}
</style>
