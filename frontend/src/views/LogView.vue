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
        <div class="flex-between mb-16">
          <div class="flex gap-8">
            <el-radio-group v-model="level" size="small" @change="loadLogs">
              <el-radio-button label="all">全部</el-radio-button>
              <el-radio-button label="info">Info</el-radio-button>
              <el-radio-button label="warn">Warn</el-radio-button>
              <el-radio-button label="error">Error</el-radio-button>
            </el-radio-group>
            <el-input
              v-model="keyword"
              placeholder="关键字"
              clearable
              size="small"
              style="width: 200px"
              @keyup.enter="loadLogs"
              @clear="loadLogs"
            />
            <el-button size="small" type="primary" @click="loadLogs">搜索</el-button>
          </div>
          <span v-if="log" class="muted">
            共 {{ log.lines.length }} 行{{ log.truncated ? '（已截断）' : '' }}
            · {{ log.path }}
          </span>
        </div>
        <div v-loading="loading" class="log-box">
          <div v-if="log && log.lines.length === 0" class="muted">无匹配日志</div>
          <div
            v-for="(l, i) in log?.lines ?? []"
            :key="i"
            class="log-line"
            :class="`lvl-${l.level}`"
          >
            <span class="lvl-tag">{{ l.level.toUpperCase() }}</span>
            <span class="lvl-ts">{{ l.ts || '-' }}</span>
            <span class="lvl-msg">{{ l.msg }}</span>
          </div>
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
import type { LogResult, AnalyzeResult } from '@/api/types'

const route = useRoute()
const jobName = computed(() => String(route.params.jobName || ''))
const project = computed(() => String(route.query.project || ''))
const survey = computed(() => String(route.query.survey || ''))
const jobDesc = computed(() => String(route.query.jobDesc || ''))

const activeTab = ref<'list' | 'log' | 'ai'>('list')
const level = ref<'all' | 'info' | 'warn' | 'error'>('all')
const keyword = ref('')
const log = ref<LogResult | null>(null)
const loading = ref(false)

const analyze = ref<AnalyzeResult | null>(null)
const analyzing = ref(false)

async function loadLogs() {
  if (activeTab.value === 'ai') return
  loading.value = true
  try {
    log.value = await getLogs(jobName.value, {
      type: activeTab.value,
      level: level.value,
      keyword: keyword.value,
      project: project.value,
      survey: survey.value,
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
      project: project.value,
      survey: survey.value,
      keyword: keyword.value,
    })
  } catch (e: any) {
    ElMessage.error(e.message || '分析失败')
  } finally {
    analyzing.value = false
  }
}

function onTabChange(name: string | number) {
  const tab = String(name)
  if (tab === 'ai') {
    if (!analyze.value) void runAnalyze()
  } else {
    void loadLogs()
  }
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
.log-box {
  max-height: 60vh;
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
  gap: 8px;
  padding: 1px 4px;
  color: #d4d4d4;
  white-space: pre-wrap;
  word-break: break-all;
}
.lvl-tag {
  flex: 0 0 50px;
  font-weight: 600;
}
.lvl-ts {
  flex: 0 0 150px;
  color: #9cdcfe;
}
.lvl-msg {
  flex: 1;
}
.lvl-info .lvl-tag { color: #6a9955; }
.lvl-warn .lvl-tag { color: #e2c08d; }
.lvl-warn { background: rgba(226, 192, 141, 0.08); }
.lvl-error .lvl-tag { color: #f48771; }
.lvl-error { background: rgba(244, 135, 113, 0.1); }
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
</style>
