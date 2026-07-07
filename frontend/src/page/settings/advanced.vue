<template>
  <div class="p-tab p-settings-advanced py-2">
    <v-form ref="form" validate-on="invalid-input" class="p-form-settings" accept-charset="UTF-8" @submit.prevent="onChange">
      <v-card flat tile class="mt-0 px-1 bg-background">
        <v-card-actions v-if="$config.values.restart">
          <v-row align="start" dense>
            <v-col cols="12" class="pa-2 text-start">
              <v-alert color="primary" icon="mdi-information" class="pa-2" type="info" variant="outlined">
                <a style="color: inherit" href="#restart">
                  {{ $gettext(`Changes to the advanced settings require a restart to take effect.`) }}
                </a>
              </v-alert>
            </v-col>
          </v-row>
        </v-card-actions>

        <v-card-title class="pb-0 text-subtitle-2">
          {{ $gettext(`Global Options`) }}
        </v-card-title>

        <v-card-actions>
          <v-row align="start" dense>
            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.Debug"
                :disabled="isDemo"
                class="ma-0 pa-0 input-debug"
                density="compact"
                color="surface-variant"
                :label="$gettext('Debug Logs')"
                :hint="$gettext('Enable debug mode to display additional logs and help with troubleshooting.')"
                prepend-icon="mdi-bug"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.Experimental"
                :disabled="isDemo"
                class="ma-0 pa-0 input-experimental"
                density="compact"
                color="surface-variant"
                :label="$gettext('Experimental Features')"
                :hint="$gettext('Enable new features that may be incomplete or unstable.')"
                prepend-icon="mdi-flask-empty"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.ReadOnly"
                :disabled="isDemo"
                class="ma-0 pa-0 input-readonly"
                density="compact"
                color="surface-variant"
                :label="$gettext('Read-Only Mode')"
                :hint="$gettext('Disable features that require write permission for the originals folder.')"
                prepend-icon="mdi-hand-back-right-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.DisableBackups"
                :disabled="isDemo"
                class="ma-0 pa-0 input-disable-backups"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable Backups')"
                :hint="$gettext('Prevent database and album backups as well as YAML sidecar files from being created.')"
                prepend-icon="mdi-shield-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.DisableWebDAV"
                :disabled="isDemo"
                class="ma-0 pa-0 input-disable-webdav"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable WebDAV')"
                :hint="$gettext('Prevent other apps from accessing PhotoPrism as a shared network drive.')"
                prepend-icon="mdi-sync-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.DisableMCP"
                :disabled="isDemo"
                class="ma-0 pa-0 input-disable-mcp"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable MCP')"
                :hint="$gettext('Disable the Model Context Protocol (MCP) API endpoint for AI agent integrations.')"
                prepend-icon="mdi-robot-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.DisableFaces"
                :disabled="isDemo"
                class="ma-0 pa-0 input-disable-faces"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable Faces')"
                :hint="$gettext('Disable all face detection and recognition features.')"
                prepend-icon="mdi-account-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.DisablePlaces"
                :disabled="isDemo"
                class="ma-0 pa-0 input-disable-places"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable Places')"
                :hint="$gettext('Disable interactive world maps and reverse geocoding.')"
                prepend-icon="mdi-map-marker-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="3">
              <v-checkbox
                v-model="settings.DisableExifTool"
                :disabled="isDemo || (!settings.Experimental && !settings.DisableExifTool)"
                class="ma-0 pa-0 input-disable-exiftool"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable ExifTool')"
                :hint="$gettext('ExifTool is required for full support of XMP metadata, videos and Live Photos.')"
                prepend-icon="mdi-movie-off-outline"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>
          </v-row>
        </v-card-actions>

        <template v-if="!settings.DisableFaces">
          <v-card-title class="pb-0 text-subtitle-2">
            {{ $gettext(`Face Recognition`) }}
          </v-card-title>

          <v-card-actions>
            <v-row align="start" dense>
              <v-col cols="12">
                <v-alert v-if="faceModelsError" color="warning" icon="mdi-alert" class="mb-2 pa-2" type="warning" variant="outlined">
                  {{ faceModelsError }}
                </v-alert>

                <v-progress-linear v-if="faceModelsBusy" indeterminate color="surface-variant" height="2" class="mb-1"></v-progress-linear>

                <v-table tile hover density="compact" class="bg-table p-face-recognition-models">
                  <thead>
                    <tr>
                      <th class="text-start">{{ $gettext(`Model`) }}</th>
                      <th class="text-start">{{ $gettext(`Status`) }}</th>
                      <th class="text-start hidden-sm-and-down">{{ $gettext(`GPU`) }}</th>
                      <th class="text-start">{{ $gettext(`Details`) }}</th>
                      <th class="text-end">{{ $gettext(`Actions`) }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="model in faceModels.models" :key="faceModelKey(model)" :class="{ 'is-active': model.active }">
                      <td>
                        <div class="font-weight-medium">{{ model.profile.name }}</div>
                        <div class="text-caption text-medium-emphasis">{{ model.profile.key }}</div>
                      </td>
                      <td class="text-no-wrap">
                        {{ faceModelStatus(model) }}
                      </td>
                      <td class="hidden-sm-and-down">
                        {{ faceModelGpu(model) }}
                      </td>
                      <td>
                        <div class="text-caption">{{ faceModelDetails(model) }}</div>
                        <div v-if="faceModelCompare[faceModelKey(model)]" class="text-caption text-medium-emphasis">
                          {{
                            $gettextInterpolate($gettext("Changed: %{n}"), {
                              n: faceModelCompare[faceModelKey(model)].changedSubjects,
                            })
                          }}
                        </div>
                      </td>
                      <td class="text-end text-no-wrap">
                        <v-btn
                          v-tooltip="$gettext('Prepare')"
                          icon="mdi-play-circle"
                          density="comfortable"
                          variant="plain"
                          color="surface-variant"
                          :aria-label="$gettext('Prepare')"
                          :disabled="!canPrepareFaceModel(model)"
                          @click.stop.prevent="onPrepareFaceModel(model)"
                        ></v-btn>
                        <v-btn
                          v-tooltip="$gettext('Compare')"
                          icon="mdi-table-eye"
                          density="comfortable"
                          variant="plain"
                          color="surface-variant"
                          :aria-label="$gettext('Compare')"
                          :disabled="!canCompareFaceModel(model)"
                          @click.stop.prevent="onCompareFaceModel(model)"
                        ></v-btn>
                        <v-btn
                          v-tooltip="$gettext('Promote')"
                          icon="mdi-check-decagram"
                          density="comfortable"
                          variant="plain"
                          color="surface-variant"
                          :aria-label="$gettext('Promote')"
                          :disabled="!canPromoteFaceModel(model)"
                          @click.stop.prevent="onPromoteFaceModel(model)"
                        ></v-btn>
                      </td>
                    </tr>
                    <tr v-if="!faceModelsBusy && faceModels.models.length === 0">
                      <td colspan="5" class="text-medium-emphasis">
                        {{ $gettext(`No face recognition models found.`) }}
                      </td>
                    </tr>
                  </tbody>
                </v-table>
              </v-col>
            </v-row>
          </v-card-actions>
        </template>

        <template v-if="!settings.DisableBackups">
          <v-card-title class="pb-0 text-subtitle-2">
            {{ $gettext(`Backup`) }}
          </v-card-title>

          <v-card-actions>
            <v-row align="start" dense>
              <v-col cols="12" sm="4">
                <v-checkbox
                  v-model="settings.BackupDatabase"
                  :disabled="isDemo || settings.BackupSchedule === ''"
                  class="ma-0 pa-0 input-backup-database"
                  density="compact"
                  color="surface-variant"
                  :label="$gettext('Database Backups')"
                  :hint="$gettext('Create regular backups based on the configured schedule.')"
                  prepend-icon="mdi-history"
                  persistent-hint
                  @update:model-value="onChange"
                >
                </v-checkbox>
              </v-col>

              <v-col cols="12" sm="4">
                <v-checkbox
                  v-model="settings.BackupAlbums"
                  :disabled="isDemo"
                  class="ma-0 pa-0 input-backup-albums"
                  density="compact"
                  color="surface-variant"
                  :label="$gettext('Album Backups')"
                  :hint="$gettext('Create YAML files to back up album metadata.')"
                  prepend-icon="mdi-image-album"
                  persistent-hint
                  @update:model-value="onChange"
                >
                </v-checkbox>
              </v-col>

              <v-col cols="12" sm="4">
                <v-checkbox
                  v-model="settings.SidecarYaml"
                  :disabled="isDemo"
                  class="ma-0 pa-0 input-sidecar-yaml"
                  density="compact"
                  color="surface-variant"
                  :label="$gettext('Sidecar Files')"
                  :hint="$gettext('Create YAML sidecar files to back up picture metadata.')"
                  prepend-icon="mdi-clipboard-file-outline"
                  persistent-hint
                  @update:model-value="onChange"
                >
                </v-checkbox>
              </v-col>
            </v-row>
          </v-card-actions>
        </template>

        <v-card-title class="pb-0 text-subtitle-2">
          {{ $gettext(`Preview Images`) }}
        </v-card-title>

        <v-card-actions class="grid">
          <v-row align="start">
            <v-col cols="12" lg="4" class="py-2">
              <v-list-subheader class="pa-0">
                {{ $gettextInterpolate($gettext("Static Size Limit: %{n}px"), { n: parseInt(settings.ThumbSize) }) }}
              </v-list-subheader>
              <v-slider v-model="settings.ThumbSize" :min="720" :max="15360" :step="4" :disabled="isDemo" hide-details class="ma-0" @end="onChange"></v-slider>
            </v-col>

            <v-col cols="12" sm="6" lg="4" class="py-2">
              <v-list-subheader class="pa-0">
                {{
                  $gettextInterpolate($gettext("Dynamic Size Limit: %{n}px"), {
                    n: parseInt(settings.ThumbSizeUncached),
                  })
                }}
              </v-list-subheader>
              <v-slider
                v-model="settings.ThumbSizeUncached"
                :min="720"
                :max="15360"
                :step="4"
                :disabled="isDemo"
                hide-details
                class="ma-0"
                @end="onChange"
              ></v-slider>
            </v-col>

            <v-col cols="12" sm="6" lg="4" class="py-2">
              <v-checkbox
                v-model="settings.ThumbUncached"
                :disabled="isDemo"
                class="ma-0 pa-0"
                density="compact"
                color="surface-variant"
                :label="$gettext('Dynamic Previews')"
                :hint="
                  $gettext(
                    'On-demand generation of thumbnails may cause high CPU and memory usage. It is not recommended for resource-constrained servers and NAS devices.'
                  )
                "
                prepend-icon="mdi-memory"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>
          </v-row>
        </v-card-actions>

        <v-card-title class="pb-0 text-subtitle-2">
          {{ $gettext(`Image Quality`) }}
        </v-card-title>

        <v-card-actions class="grid">
          <v-row align="start">
            <v-col cols="12" lg="4" class="py-2">
              <v-list-subheader class="pa-0">
                {{ $gettextInterpolate($gettext("JPEG Quality: %{n}"), { n: parseInt(settings.JpegQuality) }) }}
              </v-list-subheader>
              <v-slider v-model="settings.JpegQuality" :min="25" :max="100" :disabled="isDemo" hide-details class="ma-0" @end="onChange"></v-slider>
            </v-col>

            <v-col cols="12" sm="6" lg="4" class="py-2">
              <v-list-subheader class="pa-0">
                {{ $gettextInterpolate($gettext("JPEG Size Limit: %{n}px"), { n: parseInt(settings.JpegSize) }) }}
              </v-list-subheader>
              <v-slider v-model="settings.JpegSize" :min="720" :max="30000" :step="20" :disabled="isDemo" class="ma-0" @end="onChange"></v-slider>
            </v-col>

            <v-col cols="12" sm="6" lg="4" class="py-2">
              <v-list-subheader class="pa-0">
                {{ $gettextInterpolate($gettext("PNG Size Limit: %{n}px"), { n: parseInt(settings.PngSize) }) }}
              </v-list-subheader>
              <v-slider v-model="settings.PngSize" :min="720" :max="30000" :step="20" :disabled="isDemo" class="ma-0" @end="onChange"></v-slider>
            </v-col>
          </v-row>
        </v-card-actions>

        <v-card-title class="py-0 text-subtitle-2">
          {{ $gettext(`File Conversion`) }}
        </v-card-title>

        <v-card-actions>
          <v-row align="start" dense>
            <v-col cols="12" sm="6" lg="4">
              <v-checkbox
                v-model="settings.DisableDarktable"
                :disabled="isDemo || settings.DisableRaw"
                class="ma-0 pa-0 input-disable-darktable"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable Darktable')"
                :hint="$gettext('Don\'t use Darktable to convert RAW images.')"
                prepend-icon="mdi-raw-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="4">
              <v-checkbox
                v-model="settings.DisableRawTherapee"
                :disabled="isDemo || settings.DisableRaw"
                class="ma-0 pa-0 input-disable-rawtherapee"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable RawTherapee')"
                :hint="$gettext('Don\'t use RawTherapee to convert RAW images.')"
                prepend-icon="mdi-raw-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="4">
              <v-checkbox
                v-model="settings.RawPresets"
                :disabled="isDemo || settings.DisableRaw"
                class="ma-0 pa-0 input-raw-presets"
                density="compact"
                color="surface-variant"
                :label="$gettext('Use Presets')"
                :hint="$gettext('Enables RAW converter presets. May reduce performance.')"
                prepend-icon="mdi-circle-half-full"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="4">
              <v-checkbox
                v-model="settings.DisableImageMagick"
                :disabled="isDemo"
                class="ma-0 pa-0 input-disable-imagemagick"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable ImageMagick')"
                :hint="$gettext('Don\'t use ImageMagick to convert images.')"
                prepend-icon="mdi-auto-fix"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col cols="12" sm="6" lg="4">
              <v-checkbox
                v-model="settings.DisableFFmpeg"
                :disabled="isDemo || (!settings.Experimental && !settings.DisableFFmpeg)"
                class="ma-0 pa-0 input-disable-ffmpeg"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable FFmpeg')"
                :hint="$gettext('Disables video transcoding and thumbnail extraction.')"
                prepend-icon="mdi-video-off"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>

            <v-col v-if="isSponsor" cols="12" sm="6" lg="4">
              <v-checkbox
                v-model="settings.DisableVectors"
                :disabled="isDemo"
                class="ma-0 pa-0 input-disable-vectors"
                density="compact"
                color="surface-variant"
                :label="$gettext('Disable Vectors')"
                :hint="$gettext('Disables vector graphics support.')"
                prepend-icon="mdi-alpha-a-box"
                persistent-hint
                @update:model-value="onChange"
              >
              </v-checkbox>
            </v-col>
          </v-row>
        </v-card-actions>

        <v-card-actions v-if="!config.disable.restart" class="pt-6 d-flex flex-wrap ga-2">
          <a id="restart"></a>
          <v-btn color="highlight" :block="$vuetify.display.xs" :disabled="isDemo || !$config.values.restart" variant="flat" @click.stop.p.prevent="onRestart">
            {{ $gettext(`Restart`) }}
            <v-icon end>mdi-restart</v-icon>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-form>

    <p-about-footer></p-about-footer>
  </div>
</template>

<script>
import ConfigOptions from "model/config-options";
import FaceRecognitionModels from "model/face-recognition-models";
import * as options from "options/options";
import { restart } from "common/server";
import PAboutFooter from "component/about/footer.vue";

export default {
  name: "PSettingsAdvanced",
  components: {
    PAboutFooter,
  },
  data() {
    return {
      busy: this.$config.get("demo"),
      isDemo: this.$config.get("demo"),
      isPublic: this.$config.get("public"),
      isSponsor: this.$config.isSponsor(),
      readonly: this.$config.get("readonly"),
      config: this.$config.values,
      rtl: this.$isRtl,
      settings: new ConfigOptions(false),
      faceModelsBusy: false,
      faceModelAction: "",
      faceModelsError: "",
      faceModels: {
        activeModel: "facenet",
        models: [],
      },
      faceModelCompare: {},
      faceModelRuns: {},
      options: options,
    };
  },
  created() {
    if (this.isPublic && !this.isDemo) {
      this.$router.push({ name: "settings" });
    } else {
      this.load();
      this.loadFaceModels();
    }
  },
  methods: {
    // faceModelKey returns the canonical model key for UI actions.
    faceModelKey(model) {
      return model?.profile?.key || "";
    },
    // faceModelStatus returns the short availability status shown in the table.
    faceModelStatus(model) {
      if (model?.active) {
        return this.$gettext("Active");
      } else if (model?.available) {
        return this.$gettext("Ready");
      } else if (!model?.installed) {
        return this.$gettext("Missing");
      }

      return this.$gettext("Blocked");
    },
    // faceModelGpu returns GPU metadata for a model row.
    faceModelGpu(model) {
      if (model?.gpuName && model?.vramMiB) {
        return `${model.gpuName}, ${model.vramMiB} MiB`;
      } else if (model?.vramMiB) {
        return `${model.vramMiB} MiB`;
      } else if (model?.profile?.minVramMiB) {
        return this.$gettextInterpolate(this.$gettext("%{n} MiB required"), { n: model.profile.minVramMiB });
      }

      return this.$gettext("Default");
    },
    // faceModelDetails returns a readable reason or preparation summary.
    faceModelDetails(model) {
      const key = this.faceModelKey(model);
      const run = this.faceModelRuns[key];

      if (run?.status) {
        return this.$gettextInterpolate(this.$gettext("Prepared %{n} markers"), { n: run.markerCount || 0 });
      }

      if (!model?.reasons || model.reasons.length === 0) {
        return this.$gettext("Ready");
      }

      return model.reasons.map((reason) => this.faceModelReason(reason)).join(", ");
    },
    // faceModelReason returns a localized label for an availability reason.
    faceModelReason(reason) {
      switch (reason) {
        case "missing_model":
          return this.$gettext("Model files missing");
        case "missing_gpu":
          return this.$gettext("GPU not detected");
        case "insufficient_vram":
          return this.$gettext("GPU memory too small");
        case "missing_service_uri":
          return this.$gettext("Service URI missing");
        case "sidecar_unhealthy":
          return this.$gettext("Service unavailable");
        case "incomplete_preparation":
          return this.$gettext("Preparation incomplete");
        default:
          return reason;
      }
    },
    // isFaceModelCandidate returns true for models that need candidate state.
    isFaceModelCandidate(model) {
      return this.faceModelKey(model) !== "" && this.faceModelKey(model) !== "facenet";
    },
    // isFaceModelPrepared returns true when the UI has seen prepared candidate state.
    isFaceModelPrepared(model) {
      const key = this.faceModelKey(model);
      return this.faceModelRuns[key]?.status === "prepared" || this.faceModelCompare[key]?.prepared === true;
    },
    // isFaceModelAction returns true while a model action is in flight.
    isFaceModelAction(model, action) {
      return this.faceModelAction === `${this.faceModelKey(model)}:${action}`;
    },
    // canPrepareFaceModel reports whether the candidate can be prepared.
    canPrepareFaceModel(model) {
      return this.isFaceModelCandidate(model) && model?.available && !this.faceModelAction;
    },
    // canCompareFaceModel reports whether candidate comparison can be requested.
    canCompareFaceModel(model) {
      return this.isFaceModelCandidate(model) && !this.faceModelAction;
    },
    // canPromoteFaceModel reports whether the candidate can be promoted.
    canPromoteFaceModel(model) {
      return this.isFaceModelCandidate(model) && !model?.active && this.isFaceModelPrepared(model) && !this.faceModelAction;
    },
    // loadFaceModels refreshes face recognition model availability.
    loadFaceModels() {
      if (this.isDemo || this.isPublic) {
        return Promise.resolve();
      }

      this.faceModelsBusy = true;
      this.faceModelsError = "";

      return FaceRecognitionModels.list()
        .then((data) => {
          this.faceModels = {
            activeModel: data?.activeModel || "facenet",
            serviceUri: data?.serviceUri || "",
            gpuName: data?.gpuName || "",
            vramMiB: data?.vramMiB || 0,
            models: data?.models || [],
          };
        })
        .catch(() => {
          this.faceModelsError = this.$gettext("Face recognition models could not be loaded.");
        })
        .finally(() => {
          this.faceModelsBusy = false;
        });
    },
    // onPrepareFaceModel prepares candidate embeddings for a model.
    onPrepareFaceModel(model) {
      const key = this.faceModelKey(model);
      if (!key || !this.canPrepareFaceModel(model)) {
        return;
      }

      this.faceModelAction = `${key}:prepare`;

      FaceRecognitionModels.recognize(key)
        .then((run) => {
          this.faceModelRuns = { ...this.faceModelRuns, [key]: run };
          this.$notify.success(this.$gettext("Recognition model prepared"));
          this.faceModelAction = "";
          return this.onCompareFaceModel(model);
        })
        .finally(() => {
          this.faceModelAction = "";
        });
    },
    // onCompareFaceModel compares candidate and active recognition state.
    onCompareFaceModel(model) {
      const key = this.faceModelKey(model);
      if (!key || !this.canCompareFaceModel(model)) {
        return Promise.resolve();
      }

      this.faceModelAction = `${key}:compare`;

      return FaceRecognitionModels.compare(key)
        .then((result) => {
          this.faceModelCompare = { ...this.faceModelCompare, [key]: result };
        })
        .finally(() => {
          this.faceModelAction = "";
        });
    },
    // onPromoteFaceModel promotes a prepared candidate model.
    onPromoteFaceModel(model) {
      const key = this.faceModelKey(model);
      if (!key || !this.canPromoteFaceModel(model)) {
        return;
      }

      this.faceModelAction = `${key}:promote`;

      FaceRecognitionModels.promote(key)
        .then(() => {
          this.$notify.success(this.$gettext("Recognition model promoted"));
          this.faceModelRuns = {};
          this.faceModelCompare = {};
          return this.loadFaceModels();
        })
        .finally(() => {
          this.faceModelAction = "";
        });
    },
    onRestart() {
      this.busy = true;
      restart().finally(() => {
        this.busy = false;
      });
    },
    load() {
      if (this.busy || this.isDemo) {
        return;
      }

      this.busy = true;
      this.$notify.blockUI("busy");

      this.settings.load().finally(() => {
        this.busy = false;
        this.$notify.unblockUI();
      });
    },
    onChange() {
      if (this.busy || this.isDemo) {
        return;
      }

      this.busy = true;
      this.$notify.blockUI("busy");

      this.settings
        .save()
        .then(() => {
          this.$notify.success(this.$gettext("Changes successfully saved"));
        })
        .finally(() => {
          this.busy = false;
          this.$notify.unblockUI();
        });
    },
  },
};
</script>
