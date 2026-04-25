<template>
  <div>
    <v-btn type="button" color="primary" @click.prevent="openWidget">
  Upload Screenshot
    </v-btn>

    <div v-if="imageUrl">
      <p>Upload complete!</p>
      <img :src="imageUrl" alt="Uploaded schedule" style="max-width: 100%;" />
    </div>
  </div>
</template>

<script>
export default {
  name: "ScreenshotUpload",

  data() {
    return {
      imageUrl: null,
      widget: null,
    };
  },

  mounted() {
    this.widget = window.cloudinary.createUploadWidget(
      {
        cloudName: "dhrhj8bkh",
        uploadPreset: "overlap_upload",
        sources: ["local", "camera"],
        multiple: false,
        clientAllowedFormats: ["png", "jpg", "jpeg", "webp"],
        folder: "schedule_uploads",
      },
      (error, result) => {
        if (!error && result.event === "success") {
          this.imageUrl = result.info.secure_url;
          console.log("Cloudinary URL:", this.imageUrl);
        }
      }
    );
  },

  methods: {
    openWidget() {
      this.widget.open();
    },
  },
};
</script>