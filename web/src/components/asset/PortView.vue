<template>
  <div class="port-view">
    <ProTable
      ref="proTableRef"
      api="/asset/port/list"
      rowKey="port"
      :columns="portColumns"
      :searchItems="searchItems"
      selection
      :searchPlaceholder="searchPortPlaceholder"
      @data-changed="$emit('data-changed')"
      :searchKeys="['port']"
    >
      <template #toolbar-left>
        <el-dropdown @command="handleExport">
          <el-button type="success" size="default">
            {{ $t('common.export') }}<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="selected-ports" :disabled="selectedRows.length === 0">{{ $t('common.exportSelected') || '导出选中' }}</el-dropdown-item>
              <el-dropdown-item divided command="all-ports">{{ $t('common.exportAll') || '导出所有' }}</el-dropdown-item>
              <el-dropdown-item command="csv">{{ $t('common.exportCsv') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>

      <!-- 列: 端口 -->
      <template #port="{ row }">
        <div class="port-cell">
          <el-tag type="primary" size="large" effect="dark" class="port-tag">{{ row.port }}</el-tag>
        </div>
      </template>

      <!-- 列: 出现次数(数量) -->
      <template #assetCount="{ row }">
        <el-tag type="danger">{{ row.assetCount }}</el-tag>
      </template>

      <!-- 列: 关联服务 -->
      <template #services="{ row }">
        <div v-if="row.services && row.services.length > 0">
          <el-tag v-for="svc in row.services.slice(0, 3)" :key="svc" size="small" type="success" style="margin-right: 4px;">{{ svc }}</el-tag>
          <span v-if="row.services.length > 3" class="more-text">+{{ row.services.length - 3 }}</span>
        </div>
        <span v-else class="no-data">-</span>
      </template>

      <!-- 列: 关联资产 (Hosts) -->
      <template #hosts="{ row }">
        <div v-if="row.hosts && row.hosts.length > 0" class="host-list">
          <el-tag v-for="host in row.hosts.slice(0, 3)" :key="host" size="small" type="info" style="margin-right: 4px; margin-bottom: 4px;">{{ host }}</el-tag>
          <span v-if="row.hosts.length > 3" class="more-text">+{{ row.hosts.length - 3 }}</span>
        </div>
        <span v-else class="no-data">-</span>
      </template>

      <!-- 列: 所属组织 -->
      <template #org="{ row }">
        {{ row.orgName || $t('common.defaultOrganization') }}
      </template>

      <!-- 列: 操作 -->
      <template #operation="{ row }">
        <el-button type="primary" link size="small" @click="viewAssets(row)">查看资产</el-button>
      </template>
    </ProTable>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import request from '@/api/request'
import ProTable from '@/components/common/ProTable.vue'

const { t } = useI18n()
const emit = defineEmits(['data-changed'])

const proTableRef = ref(null)
const organizations = ref([])
const searchPortPlaceholder = computed(() => t('asset.portNumber') || '搜索端口')

const selectedRows = computed(() => proTableRef.value?.selectedRows || [])

const portColumns = computed(() => [
  { label: '端口', prop: 'port', slot: 'port', width: 120 },
  { label: '资产数量', prop: 'assetCount', slot: 'assetCount', width: 100 },
  { label: '关联服务', prop: 'services', slot: 'services', width: 180 },
  { label: '关联主机', prop: 'hosts', slot: 'hosts', minWidth: 250 },
  { label: t('domain.organization'), prop: 'orgName', slot: 'org', width: 120 },
  { label: t('common.updateTime'), prop: 'updateTime', width: 160 },
  { label: t('common.operation'), slot: 'operation', width: 100, fixed: 'right' }
])

const searchItems = computed(() => [
  { label: '端口号', prop: 'port', type: 'input', inputType: 'number' },
  { label: '主机或IP', prop: 'host', type: 'input' },
  {
    label: t('domain.organization'),
    prop: 'orgId',
    type: 'select',
    options: [
      { label: t('common.allOrganizations'), value: '' },
      ...organizations.value.map(org => ({ label: org.name, value: org.id }))
    ]
  }
])

async function loadOrganizations() {
  try {
    const res = await request.post('/organization/list', { page: 1, pageSize: 100 })
    if (res.code === 0) organizations.value = res.list || []
  } catch (e) {
    console.error(e)
  }
}

function viewAssets(row) {
  window.location.href = `/asset-management?tab=inventory&port=${encodeURIComponent(row.port)}`
}

async function handleExport(command) {
  let data = []
  let filename = ''

  if (command === 'selected-ports') {
    if (selectedRows.value.length === 0) { ElMessage.warning(t('common.pleaseSelect')); return }
    data = selectedRows.value
    filename = 'ports_selected.txt'
  } else if (command === 'csv') {
    ElMessage.info('正在获取全部数据...')
    try {
      const res = await request.post('/asset/port/list', { ...proTableRef.value?.searchForm, page: 1, pageSize: 10000 })
      if (res.code === 0) { data = res.list || [] } else { ElMessage.error('获取数据失败'); return }
    } catch (e) { ElMessage.error('获取数据失败'); return }
    if (data.length === 0) { ElMessage.warning('没有可导出的数据'); return }

    const headers = ['Port', 'AssetCount', 'Services', 'Hosts', 'Organization', 'UpdateTime']
    const csvRows = [headers.join(',')]
    for (const row of data) {
      csvRows.push([
        row.port || '',
        row.assetCount || 0,
        escapeCsvField((row.services || []).join(';')),
        escapeCsvField((row.hosts || []).join(';')),
        escapeCsvField(row.orgName || ''),
        escapeCsvField(row.updateTime || '')
      ].join(','))
    }
    const BOM = '\uFEFF'
    const blob = new Blob([BOM + csvRows.join('\n')], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `ports_${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(link); link.click(); document.body.removeChild(link)
    URL.revokeObjectURL(url)
    ElMessage.success(`成功导出 ${data.length} 条数据`)
    return
  } else {
    ElMessage.info('正在获取全部数据...')
    try {
      const res = await request.post('/asset/port/list', { ...proTableRef.value?.searchForm, page: 1, pageSize: 10000 })
      if (res.code === 0) { data = res.list || [] } else { ElMessage.error('获取数据失败'); return }
    } catch (e) { ElMessage.error('获取数据失败'); return }
    filename = 'ports_all.txt'
  }

  if (data.length === 0) { ElMessage.warning('没有可导出的数据'); return }

  const exportData = []
  for (const row of data) {
    if (row.port) { exportData.push(row.port) }
  }

  if (exportData.length === 0) { ElMessage.warning('没有可导出的数据'); return }

  const blob = new Blob([exportData.join('\n')], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url; link.download = filename
  document.body.appendChild(link); link.click(); document.body.removeChild(link)
  URL.revokeObjectURL(url)
  ElMessage.success(`成功导出 ${exportData.length} 个端口`)
}

function escapeCsvField(field) {
  if (field == null) return ''
  const str = String(field)
  if (str.includes(',') || str.includes('"') || str.includes('\n') || str.includes('\r')) {
    return '"' + str.replace(/"/g, '""') + '"'
  }
  return str
}

onMounted(() => {
  loadOrganizations()
})

function refresh() {
  proTableRef.value?.loadData()
}

defineExpose({ refresh })
</script>

<style scoped>
.port-view {
  height: 100%;
}
.port-cell {
  display: flex;
  align-items: center;
}
.port-tag {
  font-family: 'Consolas', 'Monaco', monospace;
  font-weight: bold;
}
.more-text {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-left: 4px;
}
.no-data {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
.host-list {
  display: flex;
  flex-wrap: wrap;
}
</style>
