import { describe, it, expect, beforeEach, vi } from "vitest";
import { shallowMount, flushPromises } from "@vue/test-utils";
import "../../fixtures";

const faceModelApi = vi.hoisted(() => ({
  list: vi.fn(),
  recognize: vi.fn(),
  compare: vi.fn(),
  promote: vi.fn(),
}));

vi.mock("model/face-recognition-models", () => ({
  default: faceModelApi,
}));

import PSettingsAdvanced from "page/settings/advanced.vue";

const modelList = {
  activeModel: "facenet",
  models: [
    {
      profile: { key: "facenet", name: "FaceNet" },
      installed: true,
      available: true,
      active: true,
    },
    {
      profile: { key: "auraface-v1", name: "AuraFace v1 (4x GPU)", minVramMiB: 8000 },
      installed: true,
      available: true,
      active: false,
      gpuName: "NVIDIA GeForce RTX 4060 Laptop GPU",
      vramMiB: 8188,
    },
  ],
};

function mountAdvanced() {
  return shallowMount(PSettingsAdvanced, {
    global: {
      mocks: {
        $config: {
          get: vi.fn(() => false),
          isSponsor: vi.fn(() => true),
          values: { restart: false, disable: { restart: false } },
        },
        $router: { push: vi.fn() },
        $notify: {
          blockUI: vi.fn(),
          unblockUI: vi.fn(),
          success: vi.fn(),
        },
        $gettext: (s) => s,
        $gettextInterpolate: (s, values) => s.replace("%{n}", values.n),
        $isRtl: false,
      },
      stubs: {
        "p-about-footer": true,
      },
    },
  });
}

describe("page/settings/advanced", () => {
  beforeEach(() => {
    faceModelApi.list.mockResolvedValue(modelList);
    faceModelApi.recognize.mockResolvedValue({ modelKey: "auraface-v1", status: "prepared", markerCount: 8 });
    faceModelApi.compare.mockResolvedValue({ modelKey: "auraface-v1", prepared: true, changedSubjects: 2 });
    faceModelApi.promote.mockResolvedValue({ model: "auraface-v1", status: "promoted" });
  });

  it("loads face recognition model availability", async () => {
    const wrapper = mountAdvanced();
    await flushPromises();

    expect(faceModelApi.list).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.faceModels.models).toHaveLength(2);
    expect(wrapper.vm.faceModelStatus(wrapper.vm.faceModels.models[0])).toBe("Active");
    expect(wrapper.vm.faceModelGpu(wrapper.vm.faceModels.models[1])).toContain("8188 MiB");
  });

  it("prepares, compares, and promotes a candidate model", async () => {
    const wrapper = mountAdvanced();
    await flushPromises();

    const candidate = wrapper.vm.faceModels.models[1];

    expect(wrapper.vm.canPromoteFaceModel(candidate)).toBe(false);

    await wrapper.vm.onPrepareFaceModel(candidate);
    await flushPromises();

    expect(faceModelApi.recognize).toHaveBeenCalledWith("auraface-v1");
    expect(faceModelApi.compare).toHaveBeenCalledWith("auraface-v1");
    expect(wrapper.vm.canPromoteFaceModel(candidate)).toBe(true);

    await wrapper.vm.onPromoteFaceModel(candidate);
    await flushPromises();

    expect(faceModelApi.promote).toHaveBeenCalledWith("auraface-v1");
    expect(faceModelApi.list).toHaveBeenCalledTimes(2);
  });
});
