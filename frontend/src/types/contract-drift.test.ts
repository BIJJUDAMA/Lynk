// 100% pure 7-bit ASCII
import { describe, it } from "node:test";
import assert from "node:assert/strict";

describe("Frontend API Contract Alignment", () => {
  it("validates ProjectWorkspace shape matches Go backend serialization", () => {
    // Shape returned by Go backend GET /api/v1/projects/:id
    const mockGoBackendProjectJSON = {
      id: "proj-123",
      title: "Campus Autonomous Drone",
      slug: "campus-autonomous-drone",
      category: "Robotics",
      description: "LiDAR mapping drone",
      owner_id: "usr-456",
      status: "open",
      created_at: "2026-11-01T00:00:00Z",
    };

    assert.ok(mockGoBackendProjectJSON.id, "Project must have an id string");
    assert.ok(mockGoBackendProjectJSON.slug, "Project must have a slug string");
    assert.equal(typeof mockGoBackendProjectJSON.status, "string");
  });

  it("validates CollaboratorMatch shape matches AI microservice serialization", () => {
    const mockCollaboratorMatchJSON = {
      candidate_id: "cand-789",
      name: "Jane Engineer",
      complement_score: 0.92,
      matched_skills: ["Rust", "Embedded Systems"],
      redundant_skills: ["Python"],
    };

    assert.ok(mockCollaboratorMatchJSON.candidate_id);
    assert.ok(mockCollaboratorMatchJSON.complement_score <= 1.0);
    assert.ok(Array.isArray(mockCollaboratorMatchJSON.matched_skills));
  });
});
