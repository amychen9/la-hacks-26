<template>
  <div class="tw-flex tw-items-center tw-gap-2 tw-cursor-pointer">
    <v-icon :size="isPhone ? 18 : 20" class="tw-text-green">mdi-circle-outline</v-icon>
    <span class="tw-font-semibold tw-text-very-dark-gray" :class="isPhone ? 'tw-text-base' : 'tw-text-lg'">
      {{ brandText }}
    </span>
  </div>
</template>

<script>
import { isPhone } from "@/utils"

export default {
  name: "Logo",

  props: {
    type: {
      type: "timeful" | "betterwhen2meet" | "aprilfools",
      default: "timeful",
    },
  },

  computed: {
    isPhone() {
      return isPhone(this.$vuetify)
    },
    brandText() {
      if (this.type === "betterwhen2meet" || this.type === "aprilfools") {
        return "CircleUp"
      }
      return "CircleUp"
    },
  },
}
</script>
