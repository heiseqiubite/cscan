<template>
  <el-card class="pro-table-wrapper" shadow="never">
    <!-- Toolbar area (for search, filters, actions) -->
    <div class="pro-table-toolbar">
      <div class="toolbar-left">
        <slot name="toolbar-left"></slot>

        <!-- Batch Delete Button -->
        <el-button
          v-if="batchDeleteApi"
          type="danger"
          size="small"
          :disabled="selectedRows.length === 0"
          @click="handleBatchDelete"
        >
          {{ $t('common.batchDelete') || 'Batch Delete' }} ({{ selectedRows.length }})
        </el-button>

        <!-- Export Dropdown -->
        <el-dropdown
          v-if="exportApi"
          @command="handleExport"
        >
          <el-button type="success" size="small">
            {{ $t('common.export') || 'Export' }}<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="selected" :disabled="selectedRows.length === 0">
                {{ $t('common.exportSelected') || 'Export Selected' }} ({{ selectedRows.length }})
              </el-dropdown-item>
              <el-dropdown-item divided command="all">
                {{ $t('common.exportAll') || 'Export All' }}
              </el-dropdown-item>
              <el-dropdown-item command="csv">
                {{ $t('common.exportCsv') || 'Export CSV' }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <!-- Inline stats -->
        <template v-if="statApi && Object.keys(stats).length > 0">
          <el-tag
            v-for="(label, key) in statLabels"
            :key="key"
            type="info"
            effect="plain"
            class="stat-tag"
          >
            {{ label }}: <span class="stat-value">{{ stats[key] !== undefined ? stats[key] : 0 }}</span>
          </el-tag>
        </template>
      </div>
      <div class="toolbar-right">
        <!-- Advanced search -->
        <el-popover
          v-if="searchItems && searchItems.length > 0"
          placement="bottom-end"
          :width="300"
          trigger="click"
        >
          <template #reference>
            <el-button icon="Filter">Advanced Search</el-button>
          </template>
          <el-form :model="searchForm" label-position="top" size="small">
            <el-form-item
              v-for="item in searchItems"
              :key="item.prop"
              :label="item.label"
            >
              <el-input
                v-if="item.type === 'input'"
                v-model="searchForm[item.prop]"
                :placeholder="`Enter ${item.label}`"
                clearable
              />
              <el-select
                v-else-if="item.type === 'select'"
                v-model="searchForm[item.prop]"
                :placeholder="`Select ${item.label}`"
                clearable
                style="width: 100%"
              >
                <el-option
                  v-for="opt in item.options"
                  :key="opt.value"
                  :label="opt.label"
                  :value="opt.value"
                />
              </el-select>
            </el-form-item>
            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px;">
              <el-button @click="resetSearch">Reset</el-button>
              <el-button type="primary" @click="handleSearch">Search</el-button>
            </div>
          </el-form>
        </el-popover>
        <slot name="toolbar-right"></slot>
      </div>
    </div>

    <!-- Main Table -->
    <el-table v-loading="loading" :data="tableData" v-bind="$attrs" @selection-change="handleSelectionChange">
      <el-table-column v-if="selection" type="selection" width="40" />

      <template v-for="(col, index) in columns" :key="index">
        <!-- If column has a slot -->
        <el-table-column v-if="col.slot" v-bind="col">
          <template #default="{ row }">
            <slot :name="col.slot" :row="row"></slot>
          </template>
        </el-table-column>

        <!-- Default rendering -->
        <el-table-column v-else v-bind="col">
          <template #default="{ row }">
            {{ row[col.prop] }}
          </template>
        </el-table-column>
      </template>
    </el-table>

    <!-- Pagination -->
    <div class="pro-table-footer">
      <slot name="footer">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="pagination.total"
          @size-change="loadData"
          @current-change="loadData"
        />
      </slot>
    </div>
  </el-card>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import request from '@/api/request'

defineOptions({
  name: 'ProTable',
  inheritAttrs: false
})

const props = defineProps({
  api: {
    type: String,
    default: ''
  },
  columns: {
    type: Array,
    default: () => []
  },
  statApi: {
    type: String,
    default: ''
  },
  statLabels: {
    type: Object,
    default: () => ({})
  },
  searchItems: {
    type: Array,
    default: () => []
  },
  batchDeleteApi: {
    type: String,
    default: ''
  },
  exportApi: {
    type: String,
    default: ''
  },
  rowKey: {
    type: String,
    default: 'id'
  },
  selection: {
    type: Boolean,
    default: false
  }
})

const router = useRouter()
const route = useRoute()

// State
const loading = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const stats = ref({})
const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

// I18n
const { t } = useI18n()
const emit = defineEmits(['data-changed'])

// Placeholder for advanced search form, currently synced with url
const searchForm = reactive({})

// Initialize state from URL query
function initQueryFromUrl() {
  const query = route.query
  if (query.page) pagination.page = parseInt(query.page) || 1
  if (query.pageSize) pagination.pageSize = parseInt(query.pageSize) || 10

  // Load remaining query parameters into searchForm
  for (const key in query) {
    if (key !== 'page' && key !== 'pageSize') {
      searchForm[key] = query[key]
    }
  }
}

// Sync state to URL query
function syncQueryToUrl() {
  const query = {
    page: pagination.page,
    pageSize: pagination.pageSize,
    ...searchForm
  }

  // Remove empty values to clean up URL
  Object.keys(query).forEach(key => {
    if (query[key] === '' || query[key] === null || query[key] === undefined) {
      delete query[key]
    }
  })

  router.replace({ path: route.path, query }).catch(() => {})
}

// Data fetching
async function loadStats() {
  if (!props.statApi) return
  try {
    const payload = { ...searchForm }
    const res = await request.post(props.statApi, payload)
    if (res.code === 0) {
      stats.value = res.data || {}
    }
  } catch (error) {
    console.error('Failed to load stats:', error)
  }
}

async function loadData() {
  if (props.statApi) loadStats()

  if (!props.api) return

  loading.value = true
  syncQueryToUrl()

  try {
    const payload = {
      page: pagination.page,
      pageSize: pagination.pageSize,
      ...searchForm
    }

    const res = await request.post(props.api, payload)
    if (res.code === 0) {
      tableData.value = res.list || []
      pagination.total = res.total || 0
    }
  } catch (error) {
    console.error('Failed to load table data:', error)
  } finally {
    loading.value = false
  }
}

// Reset and reload helper
function handleSearch() {
  pagination.page = 1
  loadData()
}

// Reset advanced search
function resetSearch() {
  for (const key in searchForm) {
    delete searchForm[key]
  }
  handleSearch()
}

// Selection changes
function handleSelectionChange(rows) {
  selectedRows.value = rows
}

// Batch delete
async function handleBatchDelete() {
  if (selectedRows.value.length === 0 || !props.batchDeleteApi) return

  try {
    await ElMessageBox.confirm(
      t('common.confirmBatchDelete', { count: selectedRows.value.length }) || `Are you sure you want to delete ${selectedRows.value.length} items?`,
      t('common.tip') || 'Warning',
      { type: 'warning' }
    )
  } catch (e) {
    return
  }

  const keys = selectedRows.value.map(row => row[props.rowKey])

  // The backend usually expects ips, domains, etc. Here we pass a generic key based on rowKey
  // e.g. if rowKey is 'ip', we pass { ips: [...] }
  // We'll pass both `ids` and the pluralized rowKey just to be safe, or just adapt based on how the backend expects it.
  const payloadKey = props.rowKey + 's'
  const payload = {}
  payload[payloadKey] = keys
  payload['ids'] = keys // fallback

  const res = await request.post(props.batchDeleteApi, payload)
  if (res.code === 0) {
    ElMessage.success(t('common.deleteSuccess') || 'Delete success')
    selectedRows.value = []
    loadData()
    emit('data-changed')
  }
}

// Export handling
async function handleExport(command) {
  if (!props.exportApi) return

  let data = []
  let filename = ''

  if (command === 'selected') {
    if (selectedRows.value.length === 0) {
      ElMessage.warning(t('common.pleaseSelect') || 'Please select items first')
      return
    }
    data = selectedRows.value
    filename = `export_selected_${new Date().getTime()}.txt`
  } else if (command === 'csv') {
    ElMessage.info(t('common.gettingAllData') || 'Getting all data...')
    try {
      const res = await request.post(props.exportApi || props.api, {
        ...searchForm, page: 1, pageSize: 10000
      })
      if (res.code === 0) {
        data = res.list || []
      } else {
        ElMessage.error(t('common.getDataFailed') || 'Failed to get data')
        return
      }
    } catch (e) {
      ElMessage.error(t('common.getDataFailed') || 'Failed to get data')
      return
    }

    if (data.length === 0) {
      ElMessage.warning(t('common.noDataToExport') || 'No data to export')
      return
    }

    const headers = props.columns.map(c => c.label).filter(Boolean)
    const csvRows = [headers.join(',')]

    for (const row of data) {
      const values = props.columns.filter(c => c.label).map(c => {
        let val = row[c.prop]
        if (typeof val === 'object') val = JSON.stringify(val)
        return escapeCsvField(val)
      })
      csvRows.push(values.join(','))
    }

    const BOM = '\uFEFF'
    const blob = new Blob([BOM + csvRows.join('\n')], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `export_${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)

    ElMessage.success(t('common.exportSuccess', { count: data.length }) || `Exported ${data.length} items`)
    return
  } else {
    // Export All (TXT)
    ElMessage.info(t('common.gettingAllData') || 'Getting all data...')
    try {
      const res = await request.post(props.exportApi || props.api, {
        ...searchForm, page: 1, pageSize: 10000
      })
      if (res.code === 0) {
        data = res.list || []
      } else {
        ElMessage.error(t('common.getDataFailed') || 'Failed to get data')
        return
      }
    } catch (e) {
      ElMessage.error(t('common.getDataFailed') || 'Failed to get data')
      return
    }
    filename = `export_all_${new Date().getTime()}.txt`
  }

  if (data.length === 0) {
    ElMessage.warning(t('common.noDataToExport') || 'No data to export')
    return
  }

  const seen = new Set()
  const exportData = []
  for (const row of data) {
    const keyVal = row[props.rowKey]
    if (keyVal && !seen.has(keyVal)) {
      seen.add(keyVal)
      exportData.push(keyVal)
    }
  }

  if (exportData.length === 0) {
    ElMessage.warning(t('common.noDataToExport') || 'No data to export')
    return
  }

  const blob = new Blob([exportData.join('\n')], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)

  ElMessage.success(t('common.exportSuccess', { count: exportData.length }) || `Exported ${exportData.length} items`)
}

// CSV field escaping helper
function escapeCsvField(field) {
  if (field == null) return ''
  const str = String(field)
  if (str.includes(',') || str.includes('"') || str.includes('\n') || str.includes('\r')) {
    return '"' + str.replace(/"/g, '""') + '"'
  }
  return str
}

// Handle workspace changes (reset page, reload data)
function handleWorkspaceChanged() {
  pagination.page = 1
  loadData()
}

onMounted(() => {
  initQueryFromUrl()
  loadData()
  window.addEventListener('workspace-changed', handleWorkspaceChanged)
})

onUnmounted(() => {
  window.removeEventListener('workspace-changed', handleWorkspaceChanged)
})

// Expose methods and state to parent component
defineExpose({
  tableData,
  loading,
  pagination,
  searchForm,
  loadData,
  handleSearch
})
</script>

<style scoped lang="scss">
.pro-table-wrapper {
  display: flex;
  flex-direction: column;
  height: 100%;

  :deep(.el-card__body) {
    display: flex;
    flex-direction: column;
    padding: 16px;
    height: 100%;
    box-sizing: border-box;
  }
}

.pro-table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;

  .toolbar-left, .toolbar-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }
}

.pro-table-footer {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>