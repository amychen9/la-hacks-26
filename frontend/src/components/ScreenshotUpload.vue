<template>
  <div class="tw-rounded-md tw-border tw-border-gray-200 tw-p-3 tw-space-y-3">
    <div class="tw-mb-2 tw-text-sm tw-font-medium tw-text-black">
      Upload calendar screenshot (beta)
    </div>
    <div class="tw-mb-3 tw-text-xs tw-text-dark-gray">
      Upload an image, then auto-parse availability. You can still edit manually.
    </div>

    <div class="tw-flex tw-items-center tw-gap-2">
      <v-btn
        type="button"
        color="primary"
        small
        :loading="uploading || parsing"
        @click.prevent="openWidget"
      >
        Upload Screenshot
      </v-btn>
      <span v-if="imageUrl" class="tw-text-xs tw-text-green">Uploaded</span>
    </div>

    <div v-if="imageUrl" class="tw-mt-1">
      <img
        :src="imageUrl"
        alt="Uploaded schedule"
        class="tw-max-h-44 tw-w-full tw-rounded tw-object-contain tw-bg-off-white"
      />
    </div>

    <div v-if="parseResult" class="tw-mt-1 tw-space-y-2">
      <div class="tw-text-xs tw-text-very-dark-gray">
        Parser confidence: {{ formatConfidence(parseResult.confidence) }}
      </div>
      <div class="tw-text-[11px] tw-text-dark-gray">
        Review and edit these slots before applying.
      </div>

      <div class="tw-flex tw-items-center tw-justify-between tw-gap-2">
        <div class="tw-text-xs tw-font-medium tw-text-black">Availability</div>
        <v-btn x-small text color="primary" @click="addSlot">Add slot</v-btn>
      </div>
      <div class="tw-max-h-64 tw-space-y-2 tw-overflow-y-auto tw-pr-1">
        <div
          v-for="(slot, idx) in editableAvailability"
          :key="`avail-${idx}`"
          class="tw-rounded tw-bg-light-gray tw-p-2 tw-text-xs tw-space-y-2"
        >
          <div class="tw-flex tw-items-center tw-justify-between">
            <span class="tw-text-dark-gray">Slot {{ idx + 1 }}</span>
            <v-btn x-small text color="error" @click="removeSlot(idx)">Remove</v-btn>
          </div>
          <div class="tw-grid tw-grid-cols-1 tw-gap-2 sm:tw-grid-cols-2">
            <label class="tw-flex tw-flex-col tw-gap-1">
              <span class="tw-text-[11px] tw-text-dark-gray">Start</span>
              <input
                :value="toLocalInputValue(slot.startIso)"
                type="datetime-local"
                class="tw-rounded tw-border tw-border-light-gray-stroke tw-bg-white tw-px-2 tw-py-1"
                @change="
                  (e) => updateSlot(idx, 'startIso', fromLocalInputValue(e.target.value))
                "
              />
            </label>
            <label class="tw-flex tw-flex-col tw-gap-1">
              <span class="tw-text-[11px] tw-text-dark-gray">End</span>
              <input
                :value="toLocalInputValue(slot.endIso)"
                type="datetime-local"
                class="tw-rounded tw-border tw-border-light-gray-stroke tw-bg-white tw-px-2 tw-py-1"
                @change="
                  (e) => updateSlot(idx, 'endIso', fromLocalInputValue(e.target.value))
                "
              />
            </label>
          </div>
          <div class="tw-text-dark-gray">
            Confidence: {{ formatConfidence(slot.confidence) }}
          </div>
        </div>
      </div>

      <div v-if="parseResult.ifNeeded?.length" class="tw-text-xs tw-font-medium tw-text-black">
        If Needed
      </div>
      <div
        v-for="(slot, idx) in parseResult.ifNeeded"
        :key="`if-needed-${idx}`"
        class="tw-rounded tw-bg-[#fff8e1] tw-p-2 tw-text-xs"
      >
        {{ slot.startIso }} - {{ slot.endIso }}
        <span class="tw-text-dark-gray">({{ formatConfidence(slot.confidence) }})</span>
      </div>

      <v-alert
        v-for="(warning, idx) in parseResult.warnings || []"
        :key="`warning-${idx}`"
        dense
        outlined
        type="warning"
        class="tw-text-xs"
      >
        {{ warning }}
      </v-alert>

      <v-btn small color="primary" @click="applyParsedSlots">Use these slots</v-btn>
    </div>
  </div>
</template>

<script>
import { post } from "@/utils"

export default {
  name: "ScreenshotUpload",
  props: {
    timezone: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      imageUrl: null,
      parseResult: null,
      editableAvailability: [],
      widget: null,
      uploading: false,
      parsing: false,
    }
  },
  mounted() {
    if (window.cloudinary) {
      this.widget = window.cloudinary.createUploadWidget(
        {
          cloudName: process.env.VUE_APP_CLOUDINARY_CLOUD_NAME || "dhrhj8bkh",
          uploadPreset: process.env.VUE_APP_CLOUDINARY_UPLOAD_PRESET || "overlap_upload",
          sources: ["local", "camera"],
          multiple: false,
          clientAllowedFormats: ["png", "jpg", "jpeg", "webp"],
          folder: "schedule_uploads",
        },
        this.handleUploadResult
      )
    }
  },
  methods: {
    openWidget() {
      if (!this.widget) {
        this.$emit("error", "Cloudinary widget failed to load")
        return
      }
      this.uploading = true
      this.widget.open()
    },
    async handleUploadResult(error, result) {
      if (error) {
        this.uploading = false
        this.$emit("error", "Upload failed. Please try again.")
        return
      }
      if (result.event !== "success") return

      this.uploading = false
      this.imageUrl = result.info.secure_url
      await this.parseScreenshot()
    },
    async parseScreenshot() {
      this.parsing = true
      this.parseResult = null
      const timezone =
        this.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"

      try {
        const parsed = await post("/screenshot/parse", {
          imageUrl: this.imageUrl,
          timezone,
          duration: 60,
        })
        this.parseResult = parsed
        this.editableAvailability = [...(parsed.availability || [])]
        this.$emit("parsed", parsed)
      } catch (err) {
        console.error(err)
        const message =
          err?.parsed?.message ||
          err?.message ||
          "Could not parse screenshot. Please enter times manually."
        this.$emit("error", message)
      } finally {
        this.parsing = false
      }
    },
    formatConfidence(value) {
      if (typeof value !== "number") return "N/A"
      return `${Math.round(value * 100)}%`
    },
    addSlot() {
      this.editableAvailability.push({
        startIso: new Date().toISOString(),
        endIso: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
        confidence: 0.5,
      })
    },
    removeSlot(idx) {
      this.editableAvailability.splice(idx, 1)
    },
    updateSlot(idx, key, value) {
      this.editableAvailability[idx] = {
        ...this.editableAvailability[idx],
        [key]: value,
      }
    },
    applyParsedSlots() {
      this.$emit("apply", {
        ...this.parseResult,
        availability: this.editableAvailability,
      })
    },
    toLocalInputValue(isoString) {
      if (!isoString) return ""
      const date = new Date(isoString)
      if (Number.isNaN(date.getTime())) return ""
      const tzOffsetMs = date.getTimezoneOffset() * 60000
      return new Date(date.getTime() - tzOffsetMs).toISOString().slice(0, 16)
    },
    fromLocalInputValue(localValue) {
      if (!localValue) return ""
      return new Date(localValue).toISOString()
    },
  },
}
</script>
