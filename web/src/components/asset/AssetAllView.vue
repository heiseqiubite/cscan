<template>
  <div class="asset-all-view">
    <ProTable
      ref="proTableRef"
      api="/asset/list"
      batchDeleteApi="/asset/batchDelete"
      rowKey="id"
      :columns="assetColumns"
      :searchItems="assetSearchItems"
      selection
      :searchPlaceholder="$t('asset.ipOrDomain')"
      @data-changed="handleDataChanged"
      :searchKeys="['host']"
    >
      <!-- 自定义导出(5种) -->
      <template #toolbar-left>
        <el-dropdown @command="handleExport">
          <el-button type="success" size="default">
            {{ $t('common.export') }}<el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="selected-target" :disabled="selectedRows.length === 0">{{ $t('vul.exportSelectedTargets', { count: selectedRows.length }) }}</el-dropdown-item>
              <el-dropdown-item command="selected-url" :disabled="selectedRows.length === 0">{{ $t('vul.exportSelectedUrls', { count: selectedRows.length }) }}</el-dropdown-item>
              <el-dropdown-item divided command="all-target">{{ $t('vul.exportAllTargets') }}</el-dropdown-item>
              <el-dropdown-item command="all-url">{{ $t('vul.exportAllUrls') }}</el-dropdown-item>
              <el-dropdown-item command="csv">{{ $t('common.exportCsv') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>

      <template #toolbar-right>
        <el-button type="danger" plain @click="handleClear">{{ $t('asset.clearData') }}</el-button>
      </template>

      <!-- 列: 资产(authority + orgName) -->
      <template #asset="{ row }">
        <div class="asset-cell">
          <a :href="getAssetUrl(row)" target="_blank" class="asset-link">{{ row.authority }}</a>
        </div>
        <div class="org-text">{{ row.orgName || $t('common.defaultOrganization') }}</div>
      </template>

      <!-- 列: IP + location -->
      <template #ip="{ row }">
        <div>{{ getDisplayIP(row) }}</div>
        <div v-if="row.location" class="location-text">{{ row.location }}</div>
      </template>

      <!-- 列: Port / Protocol -->
      <template #portProtocol="{ row }">
        <span class="port-text">{{ row.port > 0 ? row.port : '-' }}</span>
        <span v-if="row.service" class="service-text">{{ row.service }}</span>
      </template>

      <!-- 列: 详情(指纹) -->
      <template #detail="{ row }">
        <div class="fingerprint-list" v-if="row.app && row.app.length > 0">
          <el-tag v-for="app in (row.app || [])" :key="app" size="small" type="success" style="margin: 2px">
            {{ getAppName(app) }}
          </el-tag>
        </div>
        <span v-else class="no-data">-</span>
      </template>

      <!-- 列: 截图 -->
      <template #screenshot="{ row }">
        <el-image
          v-if="row.screenshot"
          :src="getScreenshotUrl(row.screenshot)"
          :preview-src-list="[getScreenshotUrl(row.screenshot)]"
          :z-index="9999"
          :preview-teleported="true"
          :hide-on-click-modal="true"
          fit="cover"
          class="screenshot-img"
        />
        <span v-else>-</span>
      </template>

      <!-- 列: 更新时间 + badges -->
      <template #updateTime="{ row }">
        <div>{{ row.updateTime }}</div>
        <el-tag v-if="row.isNew" type="success" size="small" effect="dark">{{ $t('common.new') }}</el-tag>
        <el-tag v-if="row.isUpdated && !row.isNew" type="warning" size="small" effect="dark" style="cursor: pointer" @click="showHistory(row)">{{ $t('asset.updated') }}</el-tag>
      </template>

      <!-- 列: 操作 -->
      <template #operation="{ row }">
        <el-button type="danger" link size="small" @click="handleDelete(row)">{{ $t('common.delete') }}</el-button>
      </template>
    </ProTable>

    <!-- 详情对话框 -->
    <el-dialog v-model="detailVisible" :title="$t('common.detail')" width="900px">
      <el-descriptions :column="2" border v-if="currentAsset">
        <el-descriptions-item :label="$t('dashboard.asset')" :span="2">
          <a :href="getAssetUrl(currentAsset)" target="_blank" class="asset-link">{{ currentAsset.authority }}</a>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('asset.host')">{{ currentAsset.host }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.port')">{{ currentAsset.port }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.service')">{{ currentAsset.service || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('task.scanType')">{{ currentAsset.scheme || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.pageTitle')" :span="2">{{ currentAsset.title || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.statusCode')">{{ currentAsset.httpStatus || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.server')">{{ currentAsset.server || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('ip.location')" :span="2">{{ currentAsset.location || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('domain.organization')">{{ currentAsset.orgName || $t('common.defaultOrganization') }}</el-descriptions-item>
        <el-descriptions-item :label="$t('common.updateTime')">{{ currentAsset.updateTime }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.fingerprint')" :span="2">
          <el-tag v-for="app in (currentAsset.app || [])" :key="app" size="small" type="success" style="margin: 2px">{{ app }}</el-tag>
          <span v-if="!(currentAsset.app || []).length">-</span>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('asset.screenshot')" :span="2" v-if="currentAsset.screenshot">
          <el-image
            :src="getScreenshotUrl(currentAsset.screenshot)"
            :preview-src-list="[getScreenshotUrl(currentAsset.screenshot)]"
            :z-index="9999"
            :preview-teleported="true"
            :hide-on-click-modal="true"
            fit="contain"
            style="max-width: 400px; max-height: 300px; cursor: pointer;"
          />
        </el-descriptions-item>
      </el-descriptions>

      <!-- 详情Tab页 -->
      <el-tabs v-model="detailActiveTab" style="margin-top: 15px" v-if="currentAsset">
        <el-tab-pane label="HTTP Header" name="header">
          <div class="detail-content-box">
            <pre v-if="currentAsset.httpHeader" class="detail-pre">{{ currentAsset.httpHeader }}</pre>
            <el-empty v-else :description="$t('common.noData')" :image-size="60" />
          </div>
        </el-tab-pane>
        <el-tab-pane label="HTTP Body" name="body">
          <div class="detail-content-box">
            <pre v-if="currentAsset.httpBody" class="detail-pre">{{ truncateBody(currentAsset.httpBody) }}</pre>
            <el-empty v-else :description="$t('common.noData')" :image-size="60" />
          </div>
        </el-tab-pane>
        <el-tab-pane label="Icon Hash" name="iconhash">
          <div class="detail-content-box">
            <div v-if="currentAsset.iconHash" class="iconhash-info">
              <div class="iconhash-value">
                <img
                  v-if="currentAsset.iconData && getIconDataUrl(currentAsset.iconData)"
                  :src="getIconDataUrl(currentAsset.iconData)"
                  class="iconhash-detail-img"
                  @error="handleIconError($event)"
                />
                <span class="label">Hash:</span>
                <el-tag type="info" size="small">{{ currentAsset.iconHash }}</el-tag>
                <el-button type="primary" link size="small" @click="copyIconHash">{{ $t('common.copy') }}</el-button>
              </div>
              <div v-if="currentAsset.iconHashFile" class="iconhash-file">
                <span class="label">{{ $t('vul.pocFile') }}:</span>
                <span>{{ currentAsset.iconHashFile }}</span>
              </div>
            </div>
            <el-empty v-else :description="$t('common.noData')" :image-size="60" />
          </div>
        </el-tab-pane>
        <el-tab-pane name="vul">
          <template #label>
            <span>{{ $t('asset.vulnerability') }}</span>
            <el-badge v-if="assetVulList.length > 0" :value="assetVulList.length" type="danger" style="margin-left: 5px" />
          </template>
          <div class="detail-content-box">
            <el-table v-if="assetVulList.length > 0" :data="assetVulList" stripe size="small" max-height="300">
              <el-table-column prop="pocFile" :label="$t('vul.vulName')" min-width="200" show-overflow-tooltip />
              <el-table-column prop="severity" :label="$t('vul.severity')" width="90">
                <template #default="{ row }">
                  <el-tag :type="getSeverityType(row.severity)" size="small">{{ row.severity }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="url" label="URL" min-width="200" show-overflow-tooltip />
              <el-table-column prop="createTime" :label="$t('common.createTime')" width="160">
                <template #default="{ row }">{{ formatTime(row.createTime) }}</template>
              </el-table-column>
            </el-table>
            <el-empty v-else :description="$t('dashboard.noVulData')" :image-size="60" />
          </div>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button @click="detailVisible = false">{{ $t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- 历史记录对话框 -->
    <el-dialog v-model="historyVisible" :title="$t('asset.scanHistory')" width="1000px">
      <div v-if="currentHistoryAsset" style="margin-bottom: 15px">
        <el-tag type="info">{{ currentHistoryAsset.authority }}</el-tag>
      </div>
      <el-table :data="historyList" v-loading="historyLoading" stripe size="small" max-height="500">
        <el-table-column prop="createTime" :label="$t('asset.scanTime')" width="160" />
        <el-table-column prop="title" :label="$t('asset.pageTitle')" min-width="120" show-overflow-tooltip />
        <el-table-column prop="httpStatus" :label="$t('asset.statusCode')" width="80" />
        <el-table-column :label="$t('asset.fingerprint')" min-width="120">
          <template #default="{ row }">
            <el-tag v-for="app in (row.app || []).slice(0, 3)" :key="app" size="small" type="success" style="margin: 2px">
              {{ getAppName(app) }}
            </el-tag>
            <span v-if="(row.app || []).length > 3" class="more-apps">+{{ (row.app || []).length - 3 }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('asset.changes')" min-width="200">
          <template #default="{ row }">
            <div v-if="row.changes && row.changes.length > 0" class="changes-cell">
              <el-tag v-for="(change, idx) in row.changes.slice(0, 3)" :key="idx" size="small" type="warning" style="margin: 2px" :title="`${change.oldValue} → ${change.newValue}`">
                {{ $t('asset.field.' + change.field, change.field) }}
              </el-tag>
              <span v-if="row.changes.length > 3" class="more-changes">+{{ row.changes.length - 3 }}</span>
            </div>
            <span v-else class="no-changes">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="100">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="showHistoryDetail(row)">{{ $t('common.detail') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!historyLoading && historyList.length === 0" :description="$t('asset.noHistory')" />
      <template #footer>
        <el-button @click="historyVisible = false">{{ $t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- 历史详情对话框 -->
    <el-dialog v-model="historyDetailVisible" :title="$t('asset.scanHistory')" width="900px">
      <el-descriptions :column="2" border size="small" v-if="currentHistoryDetail">
        <el-descriptions-item :label="$t('asset.scanTime')" :span="2">{{ currentHistoryDetail.createTime }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.pageTitle')" :span="2">{{ currentHistoryDetail.title || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.statusCode')">{{ currentHistoryDetail.httpStatus || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.service')">{{ currentHistoryDetail.service || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="$t('asset.fingerprint')" :span="2">
          <el-tag v-for="app in (currentHistoryDetail.app || [])" :key="app" size="small" type="success" style="margin: 2px">{{ app }}</el-tag>
          <span v-if="!(currentHistoryDetail.app || []).length">-</span>
        </el-descriptions-item>
      </el-descriptions>

      <el-card v-if="currentHistoryDetail && currentHistoryDetail.changes && currentHistoryDetail.changes.length > 0" style="margin-top: 15px">
        <template #header><span>{{ $t('asset.changeDetails') }}</span></template>
        <el-table :data="currentHistoryDetail.changes" stripe size="small">
          <el-table-column prop="field" :label="$t('asset.changedField')" width="120">
            <template #default="{ row }">
              <el-tag size="small">{{ $t('asset.field.' + row.field, row.field) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="oldValue" :label="$t('asset.oldValue')" min-width="200">
            <template #default="{ row }">
              <span class="change-value old-value">{{ row.oldValue || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column width="50" align="center">
            <template #default>
              <el-icon><Right /></el-icon>
            </template>
          </el-table-column>
          <el-table-column prop="newValue" :label="$t('asset.newValue')" min-width="200">
            <template #default="{ row }">
              <span class="change-value new-value">{{ row.newValue || '-' }}</span>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-tabs v-model="historyDetailTab" style="margin-top: 15px" v-if="currentHistoryDetail">
        <el-tab-pane label="Header" name="header">
          <div class="detail-content-box">
            <pre v-if="currentHistoryDetail.httpHeader" class="detail-pre">{{ currentHistoryDetail.httpHeader }}</pre>
            <el-empty v-else :description="$t('common.noData')" :image-size="60" />
          </div>
        </el-tab-pane>
        <el-tab-pane label="Body" name="body">
          <div class="detail-content-box">
            <pre v-if="currentHistoryDetail.httpBody" class="detail-pre">{{ truncateBody(currentHistoryDetail.httpBody) }}</pre>
            <el-empty v-else :description="$t('common.noData')" :image-size="60" />
          </div>
        </el-tab-pane>
        <el-tab-pane label="IconHash" name="iconhash">
          <div class="detail-content-box">
            <el-tag v-if="currentHistoryDetail.iconHash" type="info">{{ currentHistoryDetail.iconHash }}</el-tag>
            <el-empty v-else :description="$t('common.noData')" :image-size="60" />
          </div>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="historyDetailVisible = false">{{ $t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- 目录扫描详情对话框 -->
    <el-dialog v-model="dirScanDetailVisible" title="目录扫描结果" width="900px">
      <div v-if="dirScanDetailRow" style="margin-bottom: 15px">
        <el-tag type="info">{{ dirScanDetailRow.authority }}</el-tag>
        <span style="margin-left: 10px; color: var(--el-text-color-secondary)">共 {{ dirScanDetailData.length }} 条记录</span>
      </div>
      <el-table :data="dirScanDetailData" stripe size="small" max-height="500">
        <el-table-column prop="url" label="URL" min-width="300" show-overflow-tooltip>
          <template #default="{ row }">
            <a :href="row.url" target="_blank" rel="noopener" class="url-link">{{ row.url }}</a>
          </template>
        </el-table-column>
        <el-table-column prop="path" label="路径" min-width="120" show-overflow-tooltip />
        <el-table-column prop="statusCode" label="状态码" width="90">
          <template #default="{ row }">
            <el-tag :type="getDirScanStatusType(row.statusCode)" size="small">{{ row.statusCode }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="contentLength" label="大小" width="90">
          <template #default="{ row }">{{ formatDirScanSize(row.contentLength) }}</template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="120" show-overflow-tooltip />
        <el-table-column prop="createTime" label="发现时间" width="150" />
      </el-table>
      <template #footer>
        <el-button @click="dirScanDetailVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Right } from '@element-plus/icons-vue'
import { getAssetList, batchDeleteAssets, clearAssets } from '@/api/asset'
import request from '@/api/request'
import ProTable from '@/components/common/ProTable.vue'

const { t } = useI18n()
const emit = defineEmits(['data-changed'])

const proTableRef = ref(null)

const selectedRows = computed(() => proTableRef.value?.selectedRows || [])

// 组织列表
const organizations = ref([])

// 详情对话框
const detailVisible = ref(false)
const currentAsset = ref(null)
const detailActiveTab = ref('header')
const assetVulList = ref([])

// 行级漏洞/目录扫描数据
const rowVulMap = ref({})
const rowDirScanMap = ref({})

// 历史记录
const historyVisible = ref(false)
const historyLoading = ref(false)
const historyList = ref([])
const currentHistoryAsset = ref(null)
const historyDetailVisible = ref(false)
const currentHistoryDetail = ref(null)
const historyDetailTab = ref('header')

// 目录扫描详情
const dirScanDetailVisible = ref(false)
const dirScanDetailData = ref([])
const dirScanDetailRow = ref(null)

// 导入
const importDialogVisible = ref(false)
const importTargets = ref('')
const importLoading = ref(false)
const importTargetCount = computed(() => {
  if (!importTargets.value.trim()) return 0
  return importTargets.value.trim().split('\n').filter(line => line.trim()).length
})

const assetColumns = computed(() => [
  { label: t('dashboard.asset'), prop: 'authority', slot: 'asset', minWidth: 200 },
  { label: 'IP', prop: 'host', slot: 'ip', width: 140 },
  { label: t('asset.portProtocol'), prop: 'port', slot: 'portProtocol', width: 120 },
  { label: t('asset.pageTitle'), prop: 'title', minWidth: 180, showOverflowTooltip: true },
  { label: t('common.detail'), slot: 'detail', minWidth: 400 },
  { label: t('asset.screenshot'), prop: 'screenshot', slot: 'screenshot', width: 90, align: 'center' },
  { label: t('common.updateTime'), prop: 'updateTime', slot: 'updateTime', width: 160 },
  { label: t('common.operation'), slot: 'operation', width: 120, fixed: 'right' }
])

const assetSearchItems = computed(() => [
  { label: t('asset.host'), prop: 'host', type: 'input', placeholder: t('asset.ipOrDomain') },
  { label: t('asset.port'), prop: 'port', type: 'input', inputType: 'number', placeholder: t('asset.portNumber') },
  { label: t('asset.service'), prop: 'service', type: 'input', placeholder: 'http/ssh...' },
  { label: t('asset.pageTitle'), prop: 'title', type: 'input', placeholder: t('asset.webPageTitle') },
  { label: t('asset.app'), prop: 'app', type: 'input', placeholder: t('asset.fingerprintApp') },
  {
    label: t('domain.organization'),
    prop: 'orgId',
    type: 'select',
    options: [
      { label: t('common.allOrganizations'), value: '' },
      ...organizations.value.map(org => ({ label: org.name, value: org.id }))
    ]
  },
  { label: 'IconHash', prop: 'iconHash', type: 'input', placeholder: 'IconHash' }
])

// 监听 ProTable 的 tableData 变化，加载行级漏洞/目录扫描数据
watch(() => proTableRef.value?.tableData, (newData) => {
  if (newData && newData.length > 0) {
    newData.forEach(row => {
      if (!row._activeTab) {
        row._activeTab = getDefaultTab(row)
      }
    })
    loadAllRowVuls(newData)
    loadAllRowDirScans(newData)
  }
}, { deep: true })

function hasAnyTabContent(row) {
  return (row.app && row.app.length > 0)
}

function getDefaultTab(row) {
  return 'app'
}

async function loadAllRowVuls(rows) {
  rowVulMap.value = {}
  const results = await Promise.allSettled(
    rows.map(row => request.post('/vul/list', { host: row.host, port: row.port, page: 1, pageSize: 10 }).then(res => ({ id: row.id, res })))
  )
  for (const result of results) {
    if (result.status === 'fulfilled' && result.value.res.code === 0 && result.value.res.list?.length > 0) {
      rowVulMap.value[result.value.id] = result.value.res.list
    }
  }
}

function getRowVulCount(row) {
  return rowVulMap.value[row.id]?.length || 0
}

async function loadAllRowDirScans(rows) {
  rowDirScanMap.value = {}
  const results = await Promise.allSettled(
    rows.map(row => request.post('/dirscan/result/list', { authority: row.authority, page: 1, pageSize: 50 }).then(res => ({ id: row.id, res })))
  )
  for (const result of results) {
    if (result.status === 'fulfilled' && result.value.res.code === 0 && result.value.res.list?.length > 0) {
      rowDirScanMap.value[result.value.id] = result.value.res.list
    }
  }
}

async function loadOrganizations() {
  try {
    const res = await request.post('/organization/list', { page: 1, pageSize: 100 })
    if (res.code === 0) organizations.value = res.list || []
  } catch (e) { console.error(e) }
}

function handleDataChanged() {
  emit('data-changed')
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm('确定删除该资产吗？', '提示', { type: 'warning' })
    const res = await batchDeleteAssets({ ids: [row.id] })
    if (res.code === 0) {
      ElMessage.success('删除成功')
      proTableRef.value?.loadData()
      emit('data-changed')
    }
  } catch (e) {
    // cancelled
  }
}

async function handleClear() {
  try {
    await ElMessageBox.confirm('确定清空所有资产数据吗？此操作不可恢复！', '警告', {
      type: 'error', confirmButtonText: '确定清空', cancelButtonText: '取消'
    })
    const res = await clearAssets()
    if (res.code === 0) {
      ElMessage.success(res.msg || '清空成功')
      proTableRef.value?.loadData()
      emit('data-changed')
    } else {
      ElMessage.error(res.msg || '清空失败')
    }
  } catch (e) {
    if (e !== 'cancel') {
      console.error('清空资产失败:', e)
      ElMessage.error('清空资产失败')
    }
  }
}

async function showDetail(row) {
  currentAsset.value = row
  detailActiveTab.value = 'header'
  assetVulList.value = []
  detailVisible.value = true
  loadAssetVulList(row.host, row.port)
}

async function loadAssetVulList(host, port) {
  try {
    const res = await request.post('/vul/list', { host, port, page: 1, pageSize: 100 })
    if (res.code === 0) assetVulList.value = res.list || []
  } catch (e) { console.error(e) }
}

async function showHistory(row) {
  currentHistoryAsset.value = row
  historyList.value = []
  historyVisible.value = true
  historyLoading.value = true
  try {
    const res = await request.post('/asset/history', { assetId: row.id, limit: 20 })
    if (res.code === 0) historyList.value = res.list || []
  } catch (e) { console.error(e) }
  finally { historyLoading.value = false }
}

function showHistoryDetail(row) {
  currentHistoryDetail.value = row
  historyDetailTab.value = 'header'
  historyDetailVisible.value = true
}

function showDirScanDetail(row) {
  dirScanDetailRow.value = row
  dirScanDetailData.value = rowDirScanMap.value[row.id] || []
  dirScanDetailVisible.value = true
}

// 导入
function showImportDialog() {
  importTargets.value = ''
  importDialogVisible.value = true
}

function validateTargets(targets) {
  const errors = []
  const urlRegex = /^https?:\/\/.+/
  const hostPortRegex = /^[^:]+:\d+$/
  const domainRegex = /^[a-zA-Z0-9.-]+$/
  for (let i = 0; i < targets.length; i++) {
    const target = targets[i].trim()
    if (!target) continue
    const isValid = urlRegex.test(target) || hostPortRegex.test(target) || domainRegex.test(target)
    if (!isValid) errors.push(`第 ${i+1} 行: "${target}" 格式无效`)
  }
  return errors
}

async function handleImport() {
  if (!importTargets.value.trim()) { ElMessage.warning('请输入要导入的目标'); return }
  const targets = importTargets.value.trim().split('\n').filter(line => line.trim())
  if (targets.length === 0) { ElMessage.warning('请输入要导入的目标'); return }
  const errors = validateTargets(targets)
  if (errors.length > 0) {
    const errorMsg = errors.slice(0, 3).join('\n') + (errors.length > 3 ? `\n...等${errors.length}个错误` : '')
    ElMessage.error(`目标格式错误:\n${errorMsg}`)
    return
  }
  importLoading.value = true
  try {
    const res = await importAssets({ targets })
    if (res.code === 0) {
      ElMessage.success(res.msg || '导入成功')
      if (res.newCount > 0) {
        importDialogVisible.value = false
        proTableRef.value?.loadData()
        emit('data-changed', { type: 'import', newCount: res.newCount, skipCount: res.skipCount, errorCount: res.errorCount, total: res.total })
      } else {
        ElMessage.info('所有目标都已存在，未新增资产')
      }
    } else {
      ElMessage.error(res.msg || '导入失败')
    }
  } catch (e) {
    ElMessage.error('导入失败: ' + e.message)
  } finally {
    importLoading.value = false
  }
}

// 导出
async function handleExport(command) {
  let data = []
  let filename = ''

  if (command === 'selected-target' || command === 'selected-url') {
    if (selectedRows.value.length === 0) { ElMessage.warning('请先选择要导出的资产'); return }
    data = selectedRows.value
    filename = command === 'selected-target' ? 'asset_targets_selected.txt' : 'asset_urls_selected.txt'
  } else if (command === 'csv') {
    ElMessage.info('正在获取全部数据...')
    try {
      const res = await getAssetList({ ...proTableRef.value?.searchForm, page: 1, pageSize: 10000 })
      if (res.code === 0) { data = res.list || [] } else { ElMessage.error('获取数据失败'); return }
    } catch (e) { ElMessage.error('获取数据失败'); return }
    if (data.length === 0) { ElMessage.warning('没有可导出的数据'); return }

    const headers = ['Authority', 'Host', 'Port', 'Service', 'Scheme', 'Title', 'StatusCode', 'Server', 'Location', 'Apps', 'IconHash', 'Organization', 'UpdateTime']
    const csvRows = [headers.join(',')]
    for (const row of data) {
      csvRows.push([
        escapeCsvField(row.authority || ''),
        escapeCsvField(row.host || ''),
        row.port || '',
        escapeCsvField(row.service || ''),
        escapeCsvField(row.scheme || ''),
        escapeCsvField(row.title || ''),
        row.httpStatus || '',
        escapeCsvField(row.server || ''),
        escapeCsvField(row.location || ''),
        escapeCsvField((row.app || []).join(';')),
        escapeCsvField(row.iconHash || ''),
        escapeCsvField(row.orgName || ''),
        escapeCsvField(row.updateTime || '')
      ].join(','))
    }
    const BOM = '\uFEFF'
    const blob = new Blob([BOM + csvRows.join('\n')], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `assets_${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(link); link.click(); document.body.removeChild(link)
    URL.revokeObjectURL(url)
    ElMessage.success(`成功导出 ${data.length} 条数据`)
    return
  } else {
    ElMessage.info('正在获取全部数据...')
    try {
      const res = await getAssetList({ ...proTableRef.value?.searchForm, page: 1, pageSize: 10000 })
      if (res.code === 0) { data = res.list || [] } else { ElMessage.error('获取数据失败'); return }
    } catch (e) { ElMessage.error('获取数据失败'); return }
    filename = command === 'all-target' ? 'asset_targets_all.txt' : 'asset_urls_all.txt'
  }

  if (data.length === 0) { ElMessage.warning('没有可导出的数据'); return }

  const seen = new Set()
  const exportData = []
  if (command.includes('target')) {
    for (const row of data) {
      const target = row.authority || (row.host + ':' + row.port)
      if (target && !seen.has(target)) { seen.add(target); exportData.push(target) }
    }
  } else {
    for (const row of data) {
      const scheme = row.service === 'https' || row.port === 443 ? 'https' : 'http'
      const url = `${scheme}://${row.host}:${row.port}`
      if (!seen.has(url)) { seen.add(url); exportData.push(url) }
    }
  }
  if (exportData.length === 0) { ElMessage.warning('没有可导出的数据'); return }

  const blob = new Blob([exportData.join('\n')], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url; link.download = filename
  document.body.appendChild(link); link.click(); document.body.removeChild(link)
  URL.revokeObjectURL(url)
  ElMessage.success(`成功导出 ${exportData.length} 条数据`)
}

function escapeCsvField(field) {
  if (field == null) return ''
  const str = String(field)
  if (str.includes(',') || str.includes('"') || str.includes('\n') || str.includes('\r')) {
    return '"' + str.replace(/"/g, '""') + '"'
  }
  return str
}

// 辅助函数
function getAssetUrl(row) {
  const scheme = row.service === 'https' || row.port === 443 ? 'https' : 'http'
  return `${scheme}://${row.host}:${row.port}`
}

function getDisplayIP(row) {
  if (row.ip && row.ip.ipv4 && row.ip.ipv4.length > 0 && row.ip.ipv4[0].ip) return row.ip.ipv4[0].ip
  if (row.ip && row.ip.ipv6 && row.ip.ipv6.length > 0 && row.ip.ipv6[0].ip) return row.ip.ipv6[0].ip
  if (isIPAddress(row.host)) return row.host
  return '-'
}

function isIPAddress(str) {
  if (!str) return false
  const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/
  const ipv6Regex = /^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$/
  return ipv4Regex.test(str) || ipv6Regex.test(str)
}

function getAppName(app) {
  if (!app) return ''
  const idx = app.indexOf('[')
  return idx > 0 ? app.substring(0, idx) : app
}

function getScreenshotUrl(screenshot) {
  if (!screenshot) return ''
  if (screenshot.startsWith('data:') || screenshot.startsWith('/9j/') || screenshot.startsWith('iVBOR')) {
    return screenshot.startsWith('data:') ? screenshot : `data:image/png;base64,${screenshot}`
  }
  return `/api/screenshot/${screenshot}`
}

function truncateBody(body) {
  if (!body) return ''
  const maxLen = 5000
  if (body.length > maxLen) return body.substring(0, maxLen) + '\n\n... [内容过长，已截断]'
  return body
}

function truncateText(text, maxLen = 100) {
  if (!text) return ''
  if (text.length > maxLen) return text.substring(0, maxLen) + '...'
  return text
}

function getIconDataUrl(iconData) {
  if (!iconData || iconData.length === 0) return ''
  if (typeof iconData === 'string' && iconData.startsWith('data:')) return iconData
  const base64Str = typeof iconData === 'string' ? iconData : ''
  if (!base64Str) return ''
  try {
    const binaryStr = atob(base64Str)
    if (binaryStr.length < 4) return ''
    let start = 0
    while (start < binaryStr.length && (binaryStr[start] === ' ' || binaryStr[start] === '\t' || binaryStr[start] === '\n' || binaryStr[start] === '\r')) { start++ }
    if (binaryStr[start] === '<') {
      const header = binaryStr.substring(start, start + 100).toLowerCase()
      if (header.startsWith('<!doctype') || header.startsWith('<html') || header.startsWith('<?xml')) return ''
      if (header.startsWith('<svg')) return `data:image/svg+xml;base64,${base64Str}`
      return ''
    }
    const bytes = new Uint8Array(binaryStr.length)
    for (let i = 0; i < binaryStr.length; i++) { bytes[i] = binaryStr.charCodeAt(i) }
    if (bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4E && bytes[3] === 0x47) return `data:image/png;base64,${base64Str}`
    if (bytes[0] === 0xFF && bytes[1] === 0xD8 && bytes[2] === 0xFF) return `data:image/jpeg;base64,${base64Str}`
    if (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x38) return `data:image/gif;base64,${base64Str}`
    if (bytes[0] === 0x00 && bytes[1] === 0x00 && (bytes[2] === 0x01 || bytes[2] === 0x02) && bytes[3] === 0x00) return `data:image/*;base64,${base64Str}`
    if (bytes[0] === 0x42 && bytes[1] === 0x4D) return `data:image/bmp;base64,${base64Str}`
    if (bytes.length >= 12 && bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46 &&
        bytes[8] === 0x57 && bytes[9] === 0x45 && bytes[10] === 0x42 && bytes[11] === 0x50) return `data:image/webp;base64,${base64Str}`
    return ''
  } catch (e) { return '' }
}

function copyIconHash() {
  if (currentAsset.value && currentAsset.value.iconHash) {
    const hash = currentAsset.value.iconHash
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(hash).then(() => { ElMessage.success('已复制IconHash') }).catch(() => { fallbackCopyToClipboard(hash) })
    } else {
      fallbackCopyToClipboard(hash)
    }
  }
}

function fallbackCopyToClipboard(text) {
  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'; textarea.style.left = '-999999px'; textarea.style.top = '-999999px'
    document.body.appendChild(textarea); textarea.focus(); textarea.select()
    const successful = document.execCommand('copy')
    document.body.removeChild(textarea)
    if (successful) { ElMessage.success('已复制IconHash') } else { ElMessage.error('复制失败') }
  } catch (err) { console.error('复制失败:', err); ElMessage.error('复制失败') }
}

function handleIconError(event) { event.target.style.display = 'none' }

function getSeverityType(severity) {
  const map = { critical: 'danger', high: 'danger', medium: 'warning', low: 'info', info: 'success', unknown: 'info' }
  return map[severity?.toLowerCase()] || 'info'
}

function getDirScanStatusType(code) {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 300 && code < 400) return 'warning'
  if (code >= 400) return 'danger'
  return 'info'
}

function formatDirScanSize(bytes) {
  if (!bytes || bytes < 0) return '-'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

function formatTime(time) {
  if (!time) return '-'
  if (typeof time === 'string') return time
  return new Date(time).toLocaleString()
}

function handleWorkspaceChanged() {
  loadOrganizations()
}

onMounted(() => {
  loadOrganizations()
  window.addEventListener('workspace-changed', handleWorkspaceChanged)
})

onUnmounted(() => {
  window.removeEventListener('workspace-changed', handleWorkspaceChanged)
})

function refresh() {
  proTableRef.value?.loadData()
}

defineExpose({ refresh })
</script>

<style scoped lang="scss">
.asset-all-view {
  height: 100%;

  .asset-cell .asset-link { color: #409eff; text-decoration: none; &:hover { text-decoration: underline; } }
  .org-text, .location-text { color: var(--el-text-color-secondary); font-size: 12px; }
  .port-text { font-weight: 500; margin-right: 8px; }
  .service-text { color: #67c23a; font-size: 12px; }
  .more-apps { color: var(--el-text-color-secondary); font-size: 12px; margin-left: 4px; }
  .no-data { color: var(--el-text-color-placeholder); font-size: 12px; }
  .screenshot-img { width: 70px; height: 50px; border-radius: 4px; cursor: pointer; }

  /* 详情指纹列表样式 */
  .fingerprint-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    max-height: 120px;
    overflow-y: auto;
  }

  /* 详情Tab页样式 */
  .detail-content-box {
    min-height: 150px;
    max-height: 350px;
    overflow: auto;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 4px;
    padding: 10px;
    background: var(--el-fill-color-lighter);
  }
  .detail-pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
    font-family: 'Consolas', 'Monaco', monospace;
    font-size: 12px;
    line-height: 1.5;
  }
  .iconhash-info {
    .iconhash-value {
      display: flex; align-items: center; gap: 10px; margin-bottom: 10px;
      .label { color: var(--el-text-color-secondary); }
    }
    .iconhash-file {
      .label { color: var(--el-text-color-secondary); margin-right: 10px; }
    }
    .iconhash-detail-img {
      width: 32px; height: 32px; border-radius: 4px;
      border: 1px solid var(--el-border-color-lighter);
    }
  }

  .import-tips {
    color: var(--el-text-color-secondary);
    font-size: 13px;
  }

  /* 目录扫描 Tab 样式 */
  .tab-content-dirscan {
    .dirscan-path {
      font-size: 12px; color: var(--el-text-color-regular);
      overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 250px;
    }
    .dirscan-detail {
      font-size: 12px; padding: 5px 0;
      .dirscan-title, .dirscan-meta { color: var(--el-text-color-secondary); margin-top: 4px; }
    }
    .dirscan-more { margin-top: 8px; font-size: 12px; color: var(--el-text-color-secondary); }
    :deep(.el-collapse) { border: none; }
    :deep(.el-collapse-item__header) { height: 32px; line-height: 32px; font-size: 12px; background: transparent; }
    :deep(.el-collapse-item__wrap) { background: transparent; }
    :deep(.el-collapse-item__content) { padding-bottom: 8px; }
  }

  .url-link {
    color: var(--el-color-primary);
    text-decoration: none;
    &:hover { text-decoration: underline; }
  }

  /* 变更详情样式 */
  .changes-cell { display: flex; flex-wrap: wrap; }
  .more-changes { color: var(--el-text-color-secondary); font-size: 12px; margin-left: 4px; }
  .no-changes { color: var(--el-text-color-placeholder); }
  .change-value {
    font-family: 'Consolas', 'Monaco', monospace;
    font-size: 12px;
    word-break: break-all;
  }
  .old-value { color: var(--el-color-danger); }
  .new-value { color: var(--el-color-success); }
}
</style>
