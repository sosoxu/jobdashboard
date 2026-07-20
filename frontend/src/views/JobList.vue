<template>
  <div class="page">
    <div class="card mb-16">
      <el-form :inline="true" :model="filter" class="filter-form">
        <el-form-item label="状态">
          <el-select
            v-model="filter.jobStatus"
            multiple
            collapse-tags
            collapse-tags-tooltip
            placeholder="全部"
            clearable
            style="width: 180px"
          >
            <el-option v-for="o in STATUS_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="项目">
          <el-select
            v-model="filter.project"
            multiple
            collapse-tags
            collapse-tags-tooltip
            filterable
            placeholder="全部"
            clearable
            style="width: 180px"
          >
            <el-option v-for="v in filtersData.projects" :key="v" :label="v" :value="v" />
          </el-select>
        </el-form-item>
        <el-form-item label="工区">
          <el-select
            v-model="filter.survey"
            multiple
            collapse-tags
            collapse-tags-tooltip
            filterable
            placeholder="全部"
            clearable
            style="width: 180px"
          >
            <el-option v-for="v in filtersData.surveys" :key="v" :label="v" :value="v" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户">
          <el-select
            v-model="filter.userName"
            multiple
            collapse-tags
            collapse-tags-tooltip
            filterable
            placeholder="全部"
            clearable
            style="width: 180px"
          >
            <el-option v-for="v in filtersData.users" :key="v" :label="v" :value="v" />
          </el-select>
        </el-form-item>
        <el-form-item label="作业名称">
          <el-input
            v-model="filter.jobDesc"
            placeholder="名称包含，如 input"
            clearable
            style="width: 200px"
            @keyup.enter="onSearch"
          />
        </el-form-item>
        <el-form-item label="仅我的">
          <el-switch v-model="onlyMine" @change="onOnlyMineChange" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="onSearch">查询</el-button>
          <el-button :icon="RefreshLeft" @click="onReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="card">
      <div class="flex-between mb-16">
        <div class="flex gap-8">
          <el-button
            type="danger"
            plain
            :disabled="selection.length === 0"
            @click="onBatch('delete')"
          >批量终止</el-button>
          <el-button
            type="warning"
            plain
            :disabled="selection.length === 0"
            @click="onBatch('rerun')"
          >批量重试</el-button>
          <span v-if="selection.length" class="muted" style="align-self:center">
            已选 {{ selection.length }} 项 / 共 {{ total }} 条
          </span>
        </div>
        <RefreshControl v-model:interval="interval" :last-updated="lastUpdated" @refresh="refresh" />
      </div>

      <el-table
        ref="tableRef"
        :data="jobs"
        v-loading="loading"
        row-key="jobName"
        @selection-change="onSelectionChange"
        @expand-change="onExpandChange"
        stripe
      >
        <el-table-column type="selection" width="42" />
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expand-panel">
              <el-descriptions :column="3" border size="small">
                <el-descriptions-item label="作业编号">{{ row.jobName }}</el-descriptions-item>
                <el-descriptions-item label="当前状态">
                  <el-tag :type="stateMeta(row.jobStatus).type" size="small">{{ row.jobStatusLabel }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="所属用户">{{ row.userName }}</el-descriptions-item>
                <el-descriptions-item label="项目">{{ row.project }}</el-descriptions-item>
                <el-descriptions-item label="提交时间">{{ fmtTime(row.commitTime) }}</el-descriptions-item>
                <el-descriptions-item label="作业名称">{{ row.jobDesc }}</el-descriptions-item>
                <el-descriptions-item label="执行进度">
                  <el-progress :percentage="row.jobProcess" :stroke-width="14" />
                </el-descriptions-item>
                <el-descriptions-item label="工区">{{ row.survey }}</el-descriptions-item>
                <el-descriptions-item label="运行耗时">{{ fmtDuration(row.execTime) }}</el-descriptions-item>
                <el-descriptions-item label="退出码">
                  <span v-if="row.exitCode === 0" class="muted">0</span>
                  <el-tag v-else type="danger" size="small">{{ row.exitCode }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="摘要" :span="2">{{ row.summary }}</el-descriptions-item>
              </el-descriptions>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="jobDesc" label="作业名称" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="stateMeta(row.jobStatus).type" size="small">{{ row.jobStatusLabel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="userName" label="用户" width="110" />
        <el-table-column label="进度" width="140">
          <template #default="{ row }">
            <el-progress :percentage="row.jobProcess" :stroke-width="10" />
          </template>
        </el-table-column>
        <el-table-column prop="survey" label="工区" width="120" show-overflow-tooltip />
        <el-table-column prop="project" label="项目" width="120" show-overflow-tooltip />
        <el-table-column label="运行时间" width="100">
          <template #default="{ row }">{{ fmtDuration(row.execTime) }}</template>
        </el-table-column>
        <el-table-column label="提交时间" width="160">
          <template #default="{ row }">{{ fmtTime(row.commitTime) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-tooltip content="暂停/恢复接口待提供" placement="top">
              <span>
                <el-button size="small" disabled>暂停</el-button>
              </span>
            </el-tooltip>
            <el-tooltip
              :content="canControl(row as JobListItem) ? '' : '无权操作他人作业'"
              placement="top"
              :disabled="canControl(row as JobListItem)"
            >
              <span>
                <el-button
                  size="small"
                  type="danger"
                  :disabled="!canControl(row as JobListItem)"
                  @click="onControl('delete', [row.jobName])"
                >停止</el-button>
              </span>
            </el-tooltip>
            <el-tooltip
              :content="retryTooltip(row as JobListItem)"
              placement="top"
              :disabled="canControl(row as JobListItem) && canRerun(row.jobStatus)"
            >
              <span>
                <el-button
                  size="small"
                  type="warning"
                  :disabled="!canControl(row as JobListItem) || !canRerun(row.jobStatus)"
                  @click="onControl('rerun', [row.jobName])"
                >重试</el-button>
              </span>
            </el-tooltip>
            <el-button size="small" text type="primary" @click="viewLogs(row as JobListItem)">日志</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="flex-between mt-16">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="load"
          @size-change="onSizeChange"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessageBox, ElMessage } from 'element-plus'
import { Search, RefreshLeft } from '@element-plus/icons-vue'
import type { TableInstance } from 'element-plus'
import RefreshControl from '@/components/RefreshControl.vue'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { STATUS_OPTIONS, stateMeta, JobState } from '@/composables/useJobStatus'
import { getJobs, getJobFilters, controlJobs } from '@/api/job'
import type { JobListItem, JobFilters } from '@/api/types'
import { fmtTime, fmtDuration } from '@/utils/format'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()

const jobs = ref<JobListItem[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const selection = ref<JobListItem[]>([])
const tableRef = ref<TableInstance>()
const expandedJobName = ref<string>('')

// When enabled, the query is restricted to the current user's jobs,
// overriding the multi-select userName filter below.
const onlyMine = ref(false)

// Multi-select filter values. Status is number[] (enum codes); the rest are
// string[] populated from the /jobs/filters endpoint.
const filter = reactive({
  jobStatus: [] as number[],
  project: [] as string[],
  survey: [] as string[],
  userName: [] as string[],
  jobDesc: '',
})

const filtersData = ref<JobFilters>({ cacheTs: 0, projects: [], surveys: [], users: [], databases: [] })

async function loadFilters() {
  try {
    filtersData.value = await getJobFilters()
  } catch (e) {
    // Non-fatal: dropdowns just stay empty; the list still loads.
    console.error('load filters failed', e)
  }
}

async function load() {
  loading.value = true
  try {
    const userNameParam =
      onlyMine.value && userStore.username ? [userStore.username] : filter.userName
    const r = await getJobs({
      page: page.value,
      pageSize: pageSize.value,
      jobStatus: filter.jobStatus.length ? filter.jobStatus : undefined,
      project: filter.project.length ? filter.project : undefined,
      survey: filter.survey.length ? filter.survey : undefined,
      userName: userNameParam.length ? userNameParam : undefined,
      jobDesc: filter.jobDesc || undefined,
    })
    jobs.value = r.list
    total.value = r.total
  } catch (e: any) {
    ElMessage.error(e.message || '查询失败')
  } finally {
    loading.value = false
  }
}

function onSearch() {
  page.value = 1
  void load()
}
function onReset() {
  filter.jobStatus = []
  filter.project = []
  filter.survey = []
  filter.userName = []
  filter.jobDesc = ''
  onlyMine.value = false
  page.value = 1
  void load()
}
function onOnlyMineChange() {
  page.value = 1
  void load()
}
function onSizeChange() {
  page.value = 1
  void load()
}
function onSelectionChange(rows: JobListItem[]) {
  selection.value = rows
}

// Permission: only the job's owner may control (stop/rerun) it.
function canControl(row: JobListItem): boolean {
  return !!userStore.username && row.userName === userStore.username
}

// Tooltip text for the rerun button depending on why it is disabled.
function retryTooltip(row: JobListItem): string {
  if (!canControl(row)) return '无权操作他人作业'
  if (!canRerun(row.jobStatus)) return '当前状态不可重试'
  return ''
}

// Accordion: only one row expanded at a time.
// el-table emits two signatures: (row, expandedRows[]) or (row, expanded: boolean).
function onExpandChange(row: JobListItem, expanded: JobListItem[] | boolean) {
  const expandedRows: JobListItem[] = Array.isArray(expanded) ? expanded : (expanded ? [row] : [])
  if (expandedRows.length > 1) {
    const others = expandedRows.filter((r) => r.jobName !== row.jobName)
    for (const r of others) {
      tableRef.value?.toggleRowExpansion(r, false)
    }
  }
  expandedJobName.value = expandedRows.find((r) => r.jobName === row.jobName) ? row.jobName : ''
}

function canRerun(status: number): boolean {
  return status === JobState.jsFailed || status === JobState.jsCanceled
}

async function onControl(action: 'delete' | 'rerun', names: string[]) {
  const label = action === 'delete' ? '终止' : '重试'
  try {
    await ElMessageBox.confirm(`确认${label}选中的 ${names.length} 个作业？`, '提示', {
      type: action === 'delete' ? 'warning' : 'info',
    })
  } catch {
    return
  }
  try {
    const r = await controlJobs(action, names)
    const failed = r.failed?.length ?? 0
    if (failed > 0) {
      ElMessage.warning(`${label}完成：成功 ${r.success.length}，失败 ${failed}`)
    } else {
      ElMessage.success(`${label}成功：${r.success.length} 个作业`)
    }
    await load()
  } catch (e: any) {
    ElMessage.error(e.message || `${label}失败`)
  }
}

function onBatch(action: 'delete' | 'rerun') {
  // Only the current user's own jobs can be controlled; filter out jobs
  // belonging to others and warn if nothing remains.
  const own = selection.value.filter((j) => canControl(j))
  if (own.length === 0) {
    ElMessage.warning('请选择您自己的作业')
    return
  }
  const names = own.map((j) => j.jobName)
  void onControl(action, names)
}

function viewLogs(row: JobListItem) {
  router.push({
    name: 'logs',
    params: { jobName: row.jobName },
    query: { project: row.project, survey: row.survey, jobDesc: row.jobDesc },
  })
}

const { interval, lastUpdated, refresh } = useAutoRefresh(async () => {
  await Promise.all([load(), loadFilters()])
}, { defaultInterval: 60 })
</script>

<style scoped>
.filter-form {
  margin-bottom: -8px;
}
.expand-panel {
  padding: 12px 16px;
  background: #fafafa;
}
</style>
