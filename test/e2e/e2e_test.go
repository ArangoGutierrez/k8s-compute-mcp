//go:build e2e

// Copyright 2026 k8s-compute-mcp contributors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("k8s-compute-mcp E2E", func() {

	Describe("submit_mpi_job", func() {
		PIt("should submit and execute MPI job successfully (requires multi-node cluster with MPI Operator)", func() {
			By("Submitting MPI job")
			resp := mcpClient.CallTool("submit_mpi_job", map[string]interface{}{
				"code":     ReadFixture("mpi_hello.py"),
				"language": "python",
				"nodes":    2,
			})
			Expect(resp.IsError()).To(BeFalse(), "MCP call failed: %s", resp.GetErrorMessage())

			structured := resp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured["job_id"]).NotTo(BeEmpty())

			jobID := structured["job_id"].(string)
			jobType := structured["job_type"].(string)
			GinkgoWriter.Printf("Submitted MPI job: %s (type: %s)\n", jobID, jobType)

			By("Waiting for MPI job to complete")
			Eventually(func() string {
				statusResp := mcpClient.CallTool("check_status", map[string]interface{}{
					"job_id":   jobID,
					"job_type": jobType,
				})
				if statusResp.IsError() {
					return ""
				}
				structured := statusResp.GetStructuredContent()
				if structured == nil {
					return ""
				}
				phase, ok := structured["phase"].(string)
				if !ok {
					return ""
				}
				GinkgoWriter.Printf("MPI job phase: %s\n", phase)
				return phase
			}, "5m", "3s").Should(Equal("Succeeded"), "MPI job did not complete successfully")

			By("Verifying MPI job artifacts")
			artifactResp := mcpClient.CallTool("read_artifact_head", map[string]interface{}{
				"path":  "/mnt/data/mpi-output.txt",
				"lines": 10,
			})
			Expect(artifactResp.IsError()).To(BeFalse(), "artifact read failed: %s", artifactResp.GetErrorMessage())
			structured = artifactResp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured["content"]).To(ContainSubstring("Hello from rank"),
				"Expected MPI output not found")
		})
	})

	Describe("submit_monte_carlo_batch", func() {
		It("should submit and execute Monte Carlo batch successfully", func() {
			By("Submitting Monte Carlo batch")
			resp := mcpClient.CallTool("submit_monte_carlo_batch", map[string]interface{}{
				"script":   ReadFixture("monte_carlo_simple.py"),
				"replicas": 3,
			})
			Expect(resp.IsError()).To(BeFalse(), "MCP call failed: %s", resp.GetErrorMessage())

			structured := resp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured["job_id"]).NotTo(BeEmpty())

			jobID := structured["job_id"].(string)
			jobType := structured["job_type"].(string)
			GinkgoWriter.Printf("Submitted Monte Carlo batch: %s (type: %s)\n", jobID, jobType)

			By("Waiting for Monte Carlo batch to complete")
			Eventually(func() string {
				statusResp := mcpClient.CallTool("check_status", map[string]interface{}{
					"job_id":   jobID,
					"job_type": jobType,
				})
				if statusResp.IsError() {
					return ""
				}
				structured := statusResp.GetStructuredContent()
				if structured == nil {
					return ""
				}
				phase, ok := structured["phase"].(string)
				if !ok {
					return ""
				}
				GinkgoWriter.Printf("Monte Carlo batch phase: %s\n", phase)
				return phase
			}, "5m", "3s").Should(Equal("Succeeded"), "Monte Carlo batch did not complete successfully")

			By("Verifying Monte Carlo artifacts")
			// Check first replica output
			artifactResp := mcpClient.CallTool("read_artifact_head", map[string]interface{}{
				"path":  "/mnt/data/monte-carlo-0/result.txt",
				"lines": 5,
			})
			Expect(artifactResp.IsError()).To(BeFalse(), "artifact read failed: %s", artifactResp.GetErrorMessage())
			structured = artifactResp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured["content"]).To(ContainSubstring("Pi estimate"),
				"Expected Monte Carlo output not found")
		})
	})

	Describe("submit_reducer", func() {
		It("should submit and execute reducer job successfully", func() {
			// First create some data to reduce
			By("Creating test data for reducer")
			_ = mcpClient.CallTool("submit_monte_carlo_batch", map[string]interface{}{
				"script":   ReadFixture("monte_carlo_simple.py"),
				"replicas": 2,
			})
			// Wait briefly for data to be written
			Eventually(func() bool {
				resp := mcpClient.CallTool("read_artifact_head", map[string]interface{}{
					"path":  "/mnt/data/monte-carlo-0/result.txt",
					"lines": 1,
				})
				return !resp.IsError()
			}, "5m", "3s").Should(BeTrue(), "Data file not created")

			By("Submitting reducer job")
			resp := mcpClient.CallTool("submit_reducer", map[string]interface{}{
				"script": ReadFixture("reducer_sum.py"),
			})
			Expect(resp.IsError()).To(BeFalse(), "MCP call failed: %s", resp.GetErrorMessage())

			structured := resp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured["job_id"]).NotTo(BeEmpty())

			jobID := structured["job_id"].(string)
			jobType := structured["job_type"].(string)
			GinkgoWriter.Printf("Submitted reducer job: %s (type: %s)\n", jobID, jobType)

			By("Waiting for reducer job to complete")
			Eventually(func() string {
				statusResp := mcpClient.CallTool("check_status", map[string]interface{}{
					"job_id":   jobID,
					"job_type": jobType,
				})
				if statusResp.IsError() {
					return ""
				}
				structured := statusResp.GetStructuredContent()
				if structured == nil {
					return ""
				}
				phase, ok := structured["phase"].(string)
				if !ok {
					return ""
				}
				GinkgoWriter.Printf("Reducer job phase: %s\n", phase)
				return phase
			}, "5m", "3s").Should(Equal("Succeeded"), "Reducer job did not complete successfully")

			By("Verifying reducer output")
			artifactResp := mcpClient.CallTool("read_artifact_head", map[string]interface{}{
				"path":  "/mnt/data/reduced-result.txt",
				"lines": 5,
			})
			Expect(artifactResp.IsError()).To(BeFalse(), "artifact read failed: %s", artifactResp.GetErrorMessage())
			structured = artifactResp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured["content"]).To(ContainSubstring("Average Pi estimate"),
				"Expected reducer output not found")
		})
	})

	Describe("check_status", func() {
		It("should return status for existing jobs", func() {
			By("Submitting a Monte Carlo batch job for status checking")
			resp := mcpClient.CallTool("submit_monte_carlo_batch", map[string]interface{}{
				"script":   ReadFixture("monte_carlo_simple.py"),
				"replicas": 1,
			})
			Expect(resp.IsError()).To(BeFalse())
			structured := resp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			jobID := structured["job_id"].(string)
			jobType := structured["job_type"].(string)

			By("Checking job status")
			statusResp := mcpClient.CallTool("check_status", map[string]interface{}{
				"job_id":   jobID,
				"job_type": jobType,
			})
			Expect(statusResp.IsError()).To(BeFalse())
			structured = statusResp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured).To(HaveKey("phase"))
			Expect(structured).To(HaveKey("job_id"))
		})

		It("should handle non-existent jobs gracefully", func() {
			By("Checking status of non-existent job")
			resp := mcpClient.CallTool("check_status", map[string]interface{}{
				"job_id": "nonexistent-job-12345",
			})
			// Should return error for invalid job ID format
			Expect(resp.IsError()).To(BeTrue(), "Expected error for invalid job ID")
		})
	})

	Describe("read_artifact_head", func() {
		BeforeEach(func() {
			// Create a test file
			By("Creating test data file")
			resp := mcpClient.CallTool("submit_monte_carlo_batch", map[string]interface{}{
				"script":   ReadFixture("monte_carlo_simple.py"),
				"replicas": 1,
			})
			Expect(resp.IsError()).To(BeFalse())

			// Wait for completion
			structured := resp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			jobID := structured["job_id"].(string)
			jobType := structured["job_type"].(string)
			Eventually(func() string {
				statusResp := mcpClient.CallTool("check_status", map[string]interface{}{
					"job_id":   jobID,
					"job_type": jobType,
				})
				if statusResp.IsError() {
					return ""
				}
				structured := statusResp.GetStructuredContent()
				if structured == nil {
					return ""
				}
				phase, _ := structured["phase"].(string)
				return phase
			}, "5m", "3s").Should(Equal("Succeeded"))
		})

		It("should read first N lines from artifact", func() {
			By("Reading first 3 lines")
			resp := mcpClient.CallTool("read_artifact_head", map[string]interface{}{
				"path":  "/mnt/data/monte-carlo-0/result.txt",
				"lines": 3,
			})
			Expect(resp.IsError()).To(BeFalse())
			structured := resp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured).To(HaveKey("content"))

			content := structured["content"].(string)
			Expect(content).NotTo(BeEmpty())
			Expect(content).To(ContainSubstring("Pi estimate"))
		})

		It("should handle non-existent files gracefully", func() {
			By("Reading non-existent file")
			resp := mcpClient.CallTool("read_artifact_head", map[string]interface{}{
				"path":  "/mnt/data/nonexistent-file.txt",
				"lines": 10,
			})
			// Should return error
			Expect(resp.IsError()).To(BeTrue())
		})

		It("should respect lines parameter", func() {
			By("Reading with lines limit")
			resp := mcpClient.CallTool("read_artifact_head", map[string]interface{}{
				"path":  "/mnt/data/monte-carlo-0/result.txt",
				"lines": 1,
			})
			Expect(resp.IsError()).To(BeFalse())
			structured := resp.GetStructuredContent()
			Expect(structured).NotTo(BeNil())
			Expect(structured["content"]).NotTo(BeEmpty())
		})
	})
})
