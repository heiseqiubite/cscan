<template>
  <div class="asset-inventory-container">
    <el-tabs
      v-model="activeLeftTab"
      tab-position="left"
      class="asset-left-tabs"
    >
      <!-- 综合资产 (Comprehensive Assets - The Card View) -->
      <el-tab-pane name="comprehensive">
        <template #label>
          <span class="left-tab-label">
            <el-icon><Menu /></el-icon>
            {{ $t('asset.comprehensive') || '综合资产' }}
          </span>
        </template>
        <AssetInventoryCardView v-if="activeLeftTab === 'comprehensive'" />
      </el-tab-pane>

      <!-- 域名 (Domains) -->
      <el-tab-pane name="domain">
        <template #label>
          <span class="left-tab-label">
            <el-icon><Position /></el-icon>
            {{ $t('asset.domains') || '域名' }}
          </span>
        </template>
        <DomainView v-if="activeLeftTab === 'domain'" />
      </el-tab-pane>

      <!-- IP -->
      <el-tab-pane name="ip">
        <template #label>
          <span class="left-tab-label">
            <el-icon><Connection /></el-icon>
            {{ $t('asset.ips') || 'IP' }}
          </span>
        </template>
        <IPView v-if="activeLeftTab === 'ip'" />
      </el-tab-pane>

      <!-- 端口 (Ports) -->
      <el-tab-pane name="port">
        <template #label>
          <span class="left-tab-label">
            <el-icon><List /></el-icon>
            {{ $t('asset.port') || '端口' }}
          </span>
        </template>
        <PortView v-if="activeLeftTab === 'port'" />
      </el-tab-pane>

      <!-- 站点 (Sites) -->
      <el-tab-pane name="site">
        <template #label>
          <span class="left-tab-label">
            <el-icon><Monitor /></el-icon>
            {{ $t('asset.sites') || '站点' }}
          </span>
        </template>
        <SiteView v-if="activeLeftTab === 'site'" />
      </el-tab-pane>

      <!-- Icon -->
      <el-tab-pane name="icon">
        <template #label>
          <span class="left-tab-label">
            <el-icon><Cpu /></el-icon>
            {{ $t('asset.icon') || 'Icon' }}
          </span>
        </template>
        <IconView v-if="activeLeftTab === 'icon'" />
      </el-tab-pane>

      <!-- 应用 (App) -->
      <el-tab-pane name="app">
        <template #label>
          <span class="left-tab-label">
            <el-icon><CopyDocument /></el-icon>
            {{ $t('asset.app') || '应用' }}
          </span>
        </template>
        <AppView v-if="activeLeftTab === 'app'" />
      </el-tab-pane>

      <!-- 目录扫描 (Directory Scans) -->
      <el-tab-pane name="dirscan">
        <template #label>
          <span class="left-tab-label">
            <el-icon><Folder /></el-icon>
            {{ $t('asset.dirManagement') || '目录' }}
          </span>
        </template>
        <DirScanView v-if="activeLeftTab === 'dirscan'" />
      </el-tab-pane>

      <!-- JSFinder (JS敏感信息) -->
      <el-tab-pane name="jsfinder">
        <template #label>
          <span class="left-tab-label">
            <el-icon><Search /></el-icon>
            {{ $t('asset.jsfinder') || 'JS发现' }}
          </span>
        </template>
        <JSFinderView v-if="activeLeftTab === 'jsfinder'" />
      </el-tab-pane>

      <!-- 漏洞风险 (Vuls) -->
      <el-tab-pane name="vul">
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
  Monitor, Folder, Warning, Cpu, CopyDocument, Search
} from '@element-plus/icons-vue'

// 导入左侧标签页对应的各个组件
import AssetInventoryCardView from '@/components/asset/AssetInventoryCardView.vue'
import PortView from '@/components/asset/PortView.vue'
import DomainView from '@/components/asset/DomainView.vue'
import IPView from '@/components/asset/IPView.vue'
import SiteView from '@/components/asset/SiteView.vue'
import DirScanView from '@/components/asset/DirScanView.vue'
import JSFinderView from '@/components/asset/JSFinderView.vue'
import VulView from '@/components/asset/VulView.vue'
import IconView from '@/components/asset/IconView.vue'
import AppView from '@/components/asset/AppView.vue'

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

    // Style the content pane
    :deep(.el-tabs__content) {
      padding: 20px;
      height: 100%;
      overflow-y: auto;
      flex: 1;
    }
  }
}
</style>

<!-- 非 scoped 样式用于覆盖 Element Plus 默认样式 -->
<style lang="scss">
.el-tabs.asset-left-tabs .el-tabs__header.is-left {
  margin-right: 0 !important;
  background-color: hsl(var(--muted) / 0.3) !important;
  padding: 16px 0 !important;
  border-right: 1px solid hsl(var(--border)) !important;
  min-width: auto !important;
  width: auto !important;
}

.el-tabs.asset-left-tabs .el-tabs__item.is-left {
  text-align: left !important;
  justify-content: flex-start !important;
  height: 48px !important;
  line-height: 48px !important;
  padding: 0 50px 0 8px !important;
  display: flex !important;
  align-items: center !important;
  width: 100% !important;
  transition: all 0.2s;
}

.el-tabs.asset-left-tabs .el-tabs__item.is-left.is-active {
  background-color: hsl(var(--primary) / 0.1) !important;
  font-weight: 600 !important;
}

.el-tabs.asset-left-tabs .el-tabs__item.is-left:hover:not(.is-active) {
  background-color: hsl(var(--muted) / 0.8) !important;
}

.el-tabs.asset-left-tabs .el-tabs__active-bar.is-left {
  right: 0 !important;
  left: auto !important;
}

.el-tabs.asset-left-tabs .el-tabs__item.is-left .left-tab-label {
  display: flex !important;
  align-items: center !important;
  justify-content: flex-start !important;
  gap: 10px;
  font-size: 14px;
  width: 100%;
}

.el-tabs.asset-left-tabs .el-tabs__item.is-left .left-tab-label .el-icon {
  font-size: 18px !important;
}
</style>
