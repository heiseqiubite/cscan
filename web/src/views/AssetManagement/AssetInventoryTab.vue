<template>
  <div class="asset-inventory-container">
    <el-tabs
      v-model="activeLeftTab"
      tab-position="left"
      class="asset-left-tabs"
    >
      <!-- 综合资产 (Comprehensive Assets - The Card View) -->
      <el-tab-pane name="comprehensive" lazy>
        <template #label>
          <span class="left-tab-label">
            <el-icon><Menu /></el-icon>
            {{ $t('asset.comprehensive') || '综合资产' }}
          </span>
        </template>
        <AssetInventoryCardView v-if="activeLeftTab === 'comprehensive'" />
      </el-tab-pane>

      <!-- 端口 (Ports) -->
      <el-tab-pane name="port" lazy>
        <template #label>
          <span class="left-tab-label">
            <el-icon><List /></el-icon>
            {{ $t('asset.port') || '端口' }}
          </span>
        </template>
        <AssetAllView v-if="activeLeftTab === 'port'" />
      </el-tab-pane>

      <!-- 域名 (Domains) -->
      <el-tab-pane name="domain" lazy>
        <template #label>
          <span class="left-tab-label">
            <el-icon><Position /></el-icon>
            {{ $t('asset.domains') || '域名' }}
          </span>
        </template>
        <DomainView v-if="activeLeftTab === 'domain'" />
      </el-tab-pane>

      <!-- IP -->
      <el-tab-pane name="ip" lazy>
        <template #label>
          <span class="left-tab-label">
            <el-icon><Connection /></el-icon>
            {{ $t('asset.ips') || 'IP' }}
          </span>
        </template>
        <IPView v-if="activeLeftTab === 'ip'" />
      </el-tab-pane>

      <!-- 站点 (Sites) -->
      <el-tab-pane name="site" lazy>
        <template #label>
          <span class="left-tab-label">
            <el-icon><Monitor /></el-icon>
            {{ $t('asset.sites') || '站点' }}
          </span>
        </template>
        <SiteView v-if="activeLeftTab === 'site'" />
      </el-tab-pane>

      <!-- 目录扫描 (Directory Scans) -->
      <el-tab-pane name="dirscan" lazy>
        <template #label>
          <span class="left-tab-label">
            <el-icon><Folder /></el-icon>
            {{ $t('asset.dirManagement') || '目录扫描' }}
          </span>
        </template>
        <DirScanView v-if="activeLeftTab === 'dirscan'" />
      </el-tab-pane>

      <!-- 漏洞风险 (Vuls) -->
      <el-tab-pane name="vul" lazy>
        <template #label>
          <span class="left-tab-label">
            <el-icon><Warning /></el-icon>
            {{ $t('asset.vulnerability') || '漏洞风险' }}
          </span>
        </template>
        <VulView v-if="activeLeftTab === 'vul'" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Menu, List, Position, Connection,
  Monitor, Folder, Warning, Cpu
} from '@element-plus/icons-vue'

// 导入左侧标签页对应的各个组件
import AssetInventoryCardView from '@/components/asset/AssetInventoryCardView.vue'
import AssetAllView from '@/components/asset/AssetAllView.vue'
import DomainView from '@/components/asset/DomainView.vue'
import IPView from '@/components/asset/IPView.vue'
import SiteView from '@/components/asset/SiteView.vue'
import DirScanView from '@/components/asset/DirScanView.vue'
import VulView from '@/components/asset/VulView.vue'

const route = useRoute()
const router = useRouter()

// 当前激活的左侧标签页，如果有 URL 参数则优先使用，否则默认为综合资产
const activeLeftTab = ref(route.query.subTab || 'comprehensive')

// 监听左侧标签页变化，同步到 URL query 参数
watch(activeLeftTab, (newTab) => {
  if (route.query.subTab !== newTab) {
    router.replace({
      query: { ...route.query, subTab: newTab }
    })
  }
}, { immediate: true })

// 监听路由参数，支持从外部通过浏览器前进/后退按钮切换
watch(() => route.query.subTab, (newTab) => {
  if (newTab && newTab !== activeLeftTab.value) {
    activeLeftTab.value = newTab
  }
})
</script>

<style lang="scss" scoped>
.asset-inventory-container {
  background: hsl(var(--card));
  border-radius: 8px;
  border: 1px solid hsl(var(--border));
  overflow: hidden;
  min-height: calc(100vh - 200px);

  .asset-left-tabs {
    height: 100%;

    // Style the left pane
    :deep(.el-tabs__header.is-left) {
      margin-right: 0;
      background-color: hsl(var(--muted) / 0.3);
      padding: 16px 0;
      border-right: 1px solid hsl(var(--border));
      min-width: 160px;
    }

    :deep(.el-tabs__active-bar) {
      right: 0;
      left: auto !important;
    }

    :deep(.el-tabs__item.is-left) {
      text-align: left;
      justify-content: flex-start;
      height: 48px;
      line-height: 48px;
      padding: 0 20px;
      display: flex;
      align-items: center;
      transition: all 0.2s;

      &.is-active {
        background-color: hsl(var(--primary) / 0.1);
        font-weight: 600;
      }

      &:hover:not(.is-active) {
        background-color: hsl(var(--muted) / 0.8);
      }
    }

    // Style the content pane
    :deep(.el-tabs__content) {
      padding: 20px;
      height: 100%;
      overflow-y: auto;
      flex: 1;
    }

    // Custom label inside tab
    .left-tab-label {
      display: flex;
      align-items: center;
      gap: 10px;
      font-size: 14px;

      .el-icon {
        font-size: 18px;
      }
    }
  }
}
</style>
