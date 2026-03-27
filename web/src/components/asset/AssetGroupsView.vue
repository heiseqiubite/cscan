<template>
  <div class="asset-groups-view">
    <ProTable
      ref="proTableRef"
      api="/asset/groups"
      rowKey="domain"
      :columns="groupColumns"
      :searchItems="groupSearchItems"
      :searchPlaceholder="$t('asset.searchAssetGroups')"
      @data-changed="$emit('data-changed')"
    >
      <!-- 资产分组名称 -->
      <template #groupName="{ row }">
        <div class="group-name">
          <el-icon class="group-icon"><FolderOpened /></el-icon>
          <span class="name-text">{{ row.domain }}</span>
          <el-tag size="small" type="info" class="count-badge">{{ row.totalServices }}</el-tag>
        </div>
      </template>

      <!-- 来源 -->
      <template #source="{ row }">
        <el-tag size="small" type="success">
          <el-icon><Compass /></el-icon>
          {{ row.source }}
        </el-tag>
      </template>

      <!-- 服务数 -->
      <template #totalServices="{ row }">
        <span class="service-count">{{ row.totalServices }} {{ $t('asset.services') }}</span>
      </template>

      <!-- 操作 -->
      <template #operation="{ row }">
        <el-button type="primary" size="small" @click="viewGroupDetails(row)">
          {{ $t('asset.scan') }}
        </el-button>
        <el-dropdown trigger="click" @command="handleCommand($event, row)">
          <el-button text>
            <el-icon><MoreFilled /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="view">{{ $t('asset.viewDetails') }}</el-dropdown-item>
              <el-dropdown-item command="export">{{ $t('asset.export') }}</el-dropdown-item>
              <el-dropdown-item command="delete" divided>{{ $t('asset.delete') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>
    </ProTable>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { FolderOpened, Compass, MoreFilled } from '@element-plus/icons-vue'
import { deleteAssetGroup } from '@/api/asset'
import ProTable from '@/components/common/ProTable.vue'

const { t } = useI18n()
const emit = defineEmits(['view-details', 'data-changed'])

const proTableRef = ref(null)

const groupColumns = computed(() => [
  { label: t('asset.assetGroupName'), prop: 'domain', slot: 'groupName', minWidth: 200 },
  { label: t('asset.source'), prop: 'source', slot: 'source', width: 150 },
  { label: t('asset.totalServices'), prop: 'totalServices', slot: 'totalServices', width: 120 },
  { label: t('asset.duration'), prop: 'duration', width: 120 },
  { label: t('asset.lastUpdated'), prop: 'lastUpdated', width: 150 },
  { label: t('common.operation'), slot: 'operation', width: 120, fixed: 'right' }
])

const groupSearchItems = computed(() => [
  { label: t('asset.assetGroupName'), prop: 'domain', type: 'input' }
])

function viewGroupDetails(row) {
  emit('view-details', row)
}

function handleCommand(command, row) {
  switch (command) {
    case 'view':
      viewGroupDetails(row)
      break
    case 'export':
      exportGroupData(row)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}

function exportGroupData(row) {
  try {
    ElMessage.info(t('asset.preparingExport'))
    const headers = [t('asset.domain'), t('asset.totalServices'), t('asset.status'), t('asset.duration'), t('asset.lastUpdated')]
    let csvContent = '\uFEFF'
    csvContent += headers.join(',') + '\n'
    csvContent += [
      row.domain,
      row.totalServices || 0,
      row.status || '',
      row.duration || '',
      row.lastUpdated || ''
    ].join(',') + '\n'

    const now = new Date()
    const filename = `asset_group_${row.domain}_${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}.csv`
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
    ElMessage.success(t('asset.exportSuccess', { count: 1 }))
  } catch (error) {
    console.error('导出失败:', error)
    ElMessage.error(t('asset.exportFailed'))
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(t('asset.confirmDeleteGroup', { name: row.domain }), t('common.tip'), {
      type: 'warning'
    })
    const res = await deleteAssetGroup({ domain: row.domain })
    if (res.code === 0) {
      ElMessage.success(t('common.deleteSuccess'))
      proTableRef.value?.loadData()
      emit('data-changed')
    } else {
      ElMessage.error(res.msg || t('common.deleteFailed'))
    }
  } catch (e) {
    // cancelled
  }
}

function refresh() {
  proTableRef.value?.loadData()
}

defineExpose({ refresh })
</script>

<style scoped lang="scss">
.asset-groups-view {
  height: 100%;

  .group-name {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .group-icon {
    color: var(--el-color-primary);
    font-size: 18px;
  }

  .name-text {
    font-weight: 500;
  }

  .count-badge {
    margin-left: auto;
  }

  .service-count {
    color: var(--el-text-color-regular);
    font-size: 13px;
  }
}
</style>
