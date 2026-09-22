<template>
  <svg
    class="ambient-lines"
    viewBox="0 0 1440 900"
    preserveAspectRatio="xMidYMid slice"
    aria-hidden="true"
  >
    <g class="drift-a">
      <path class="hair" d="M-140 150 C 260 88, 660 214, 1580 118" />
      <path class="hair" d="M-140 330 C 300 268, 720 394, 1580 298" />
      <path class="hair" d="M-140 520 C 340 458, 740 584, 1580 488" />
      <path class="hair" d="M-140 726 C 300 664, 740 790, 1580 694" />
    </g>
    <g class="drift-b">
      <path class="hair" d="M-140 240 C 320 178, 700 324, 1580 208" />
      <path class="hair" d="M-140 430 C 300 368, 720 494, 1580 398" />
      <path class="hair" d="M-140 622 C 280 560, 720 686, 1580 590" />
      <path class="glint" d="M-140 240 C 320 178, 700 324, 1580 208" />
      <path class="glint slow" d="M-140 622 C 280 560, 720 686, 1580 590" />
      <path class="glint late" d="M-140 150 C 260 88, 660 214, 1580 118" />
    </g>
  </svg>
</template>

<script setup lang="ts">
// 纯展示组件：全局背景的发丝级动态线条。
// 颜色取自 style.css 中的 --bgline / --bgline-gold，随 .dark 主题自动切换；
// prefers-reduced-motion 时动画由 CSS 停掉。
</script>
