import { describe, it, expect, beforeEach } from "vitest";
import "../fixtures";
import { Mock } from "../fixtures";
import FaceRecognitionModels from "model/face-recognition-models";

describe("model/face-recognition-models", () => {
  beforeEach(() => {
    Mock.history.get.length = 0;
    Mock.history.post.length = 0;
  });

  it("lists face recognition models", async () => {
    Mock.onGet("api/v1/faces/models").replyOnce(200, {
      activeModel: "facenet",
      models: [{ profile: { key: "facenet", name: "FaceNet" }, installed: true, available: true, active: true }],
    });

    const response = await FaceRecognitionModels.list();

    expect(response.activeModel).toBe("facenet");
    expect(response.models[0].profile.key).toBe("facenet");
    expect(Mock.history.get[0].url).toBe("faces/models");
  });

  it("prepares, compares, and promotes a model", async () => {
    Mock.onPost("api/v1/faces/models/auraface-v1/recognize").replyOnce(200, {
      modelKey: "auraface-v1",
      status: "prepared",
      markerCount: 4,
    });
    Mock.onGet("api/v1/faces/models/auraface-v1/compare").replyOnce(200, {
      modelKey: "auraface-v1",
      prepared: true,
    });
    Mock.onPost("api/v1/faces/models/auraface-v1/promote").replyOnce(200, {
      model: "auraface-v1",
      status: "promoted",
    });

    const run = await FaceRecognitionModels.recognize("auraface-v1");
    const comparison = await FaceRecognitionModels.compare("auraface-v1");
    const promotion = await FaceRecognitionModels.promote("auraface-v1");

    expect(run.status).toBe("prepared");
    expect(comparison.prepared).toBe(true);
    expect(promotion.status).toBe("promoted");
  });
});
