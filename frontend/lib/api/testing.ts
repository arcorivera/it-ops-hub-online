import { api } from "./client";

export type TestCaseStatus = "NOT_STARTED" | "PASS" | "FAIL" | "BLOCKED";
export type DeploymentStage = "PRE_PROD" | "PRODUCTION";
export type ValidationResult = "PENDING" | "PASSED" | "FAILED";

export interface TestCase {
  id: string;
  ticketId: string;
  description: string;
  environment: string;
  preconditions: string;
  expectedResult: string;
  actualResult: string;
  testerId: string | null;
  status: TestCaseStatus;
  executedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Deployment {
  id: string;
  ticketId: string;
  stage: DeploymentStage;
  version: string;
  deploymentReference: string;
  deployedBy: string | null;
  deployedAt: string;
  validationResult: ValidationResult;
  rollbackReason: string;
}

export const testingApi = {
  listTestCases: (ticketId: string) => api.get<TestCase[]>(`/api/v1/tickets/${ticketId}/test-cases`),
  createTestCase: (
    ticketId: string,
    payload: { description: string; environment?: string; preconditions?: string; expectedResult?: string }
  ) => api.post<TestCase>(`/api/v1/tickets/${ticketId}/test-cases`, payload),
  execute: (ticketId: string, caseId: string, payload: { status: TestCaseStatus; actualResult?: string; comment?: string }) =>
    api.post<TestCase>(`/api/v1/tickets/${ticketId}/test-cases/${caseId}/execute`, payload),

  listDeployments: (ticketId: string) => api.get<Deployment[]>(`/api/v1/tickets/${ticketId}/deployments`),
  createDeployment: (
    ticketId: string,
    payload: { stage: DeploymentStage; version: string; deploymentReference?: string; validationResult?: ValidationResult }
  ) => api.post<Deployment>(`/api/v1/tickets/${ticketId}/deployments`, payload),
  updateValidation: (
    ticketId: string,
    deploymentId: string,
    payload: { validationResult: ValidationResult; rollbackReason?: string }
  ) => api.post<Deployment>(`/api/v1/tickets/${ticketId}/deployments/${deploymentId}/validation`, payload),
};
