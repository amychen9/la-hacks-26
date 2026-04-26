<template>
  <div class="tw-mx-auto tw-my-6 tw-max-w-4xl tw-space-y-4 tw-px-4">
    <h1 class="tw-text-2xl tw-font-semibold tw-text-dark-green">
      Fetch.ai Agent Demo
    </h1>
    <p class="tw-text-sm tw-text-dark-gray">
      Run CircleUp's Agentverse-compatible chat endpoint and view ranked slots +
      nearby place suggestions.
    </p>

    <div class="tw-rounded-md tw-bg-[#f3f3f366] tw-p-4 tw-space-y-3">
      <div class="tw-grid tw-grid-cols-1 tw-gap-3 sm:tw-grid-cols-2">
        <label class="tw-text-xs tw-font-medium tw-text-dark-gray">
          Session ID
          <input
            v-model="sessionId"
            class="tw-mt-1 tw-w-full tw-rounded tw-border tw-border-light-gray-stroke tw-bg-white tw-px-2 tw-py-2 tw-text-sm"
          />
        </label>

        <label class="tw-text-xs tw-font-medium tw-text-dark-gray">
          Meeting Type
          <select
            v-model="meetingType"
            class="tw-mt-1 tw-w-full tw-rounded tw-border tw-border-light-gray-stroke tw-bg-white tw-px-2 tw-py-2 tw-text-sm"
          >
            <option value="study">study</option>
            <option value="social">social</option>
            <option value="work">work</option>
          </select>
        </label>
      </div>

      <label class="tw-block tw-text-xs tw-font-medium tw-text-dark-gray">
        Location Hint
        <input
          v-model="locationHint"
          class="tw-mt-1 tw-w-full tw-rounded tw-border tw-border-light-gray-stroke tw-bg-white tw-px-2 tw-py-2 tw-text-sm"
        />
      </label>

      <div class="tw-flex tw-flex-wrap tw-gap-2">
        <v-btn small @click="useMyLocation">Use my location</v-btn>
        <v-btn color="primary" small :loading="loading" @click="runAgent"
          >Run agent</v-btn
        >
      </div>

      <div v-if="currentLocation" class="tw-text-xs tw-text-dark-gray">
        Location: {{ currentLocation.latitude.toFixed(4) }},
        {{ currentLocation.longitude.toFixed(4) }}
      </div>
      <div v-if="error" class="tw-text-xs tw-text-red">
        {{ error }}
      </div>
    </div>

    <div v-if="response" class="tw-rounded-md tw-bg-[#f3f3f366] tw-p-4 tw-space-y-3">
      <div class="tw-text-sm tw-font-medium tw-text-dark-green">
        {{ response.reply }}
      </div>
      <div class="tw-text-xs tw-text-dark-gray">
        {{ response.recommendation?.logisticsSuggestion?.suggestion }}
      </div>
      <div class="tw-space-y-1">
        <div
          v-for="(place, idx) in response.recommendation?.logisticsSuggestion
            ?.nearbyPlaces || []"
          :key="idx"
          class="tw-rounded tw-bg-white tw-p-2 tw-text-xs tw-text-dark-gray"
        >
          <span class="tw-font-semibold">{{ place.name }}</span> ({{
            place.category
          }})
          - {{ place.distanceKm }} km
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { getCurrentBrowserLocation, post } from "@/utils"

export default {
  name: "FetchDemo",
  data() {
    return {
      sessionId: "69edc3f8d816c394743d47e3",
      meetingType: "study",
      locationHint: "UCLA",
      currentLocation: null,
      loading: false,
      error: "",
      response: null,
    }
  },
  methods: {
    async useMyLocation() {
      this.error = ""
      const allowLocation = window.confirm(
        "Allow CircleUp to access your current location for nearby place recommendations?"
      )
      if (!allowLocation) {
        this.error = "Location request cancelled."
        return
      }
      try {
        const loc = await getCurrentBrowserLocation()
        this.currentLocation = {
          latitude: loc.latitude,
          longitude: loc.longitude,
        }
      } catch (err) {
        this.error = err?.message || "Could not access browser location."
      }
    },
    async runAgent() {
      this.loading = true
      this.error = ""
      this.response = null
      try {
        const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"
        const payload = {
          message: "Find the best meeting slot and nearest place.",
          context: {
            sessionId: this.sessionId,
            meetingType: this.meetingType,
            locationHint: this.locationHint,
            durationMinutes: 60,
            timezone,
            currentLocation: this.currentLocation,
          },
        }
        this.response = await post("/fetch/chat", payload)
      } catch (err) {
        this.error =
          err?.parsed?.message ||
          err?.message ||
          "Failed to call fetch agent endpoint."
      } finally {
        this.loading = false
      }
    },
  },
}
</script>
