import { expect, test } from "@playwright/test";
import { clinicalReviewHref } from "../src/features/clinical-analysis/components/clinical-journey-nav";

test("clinical journey links only to a canonical patient ID", () => {
  expect(clinicalReviewHref("eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11")).toBe("/clients/eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11/clinical");
  expect(clinicalReviewHref("<svg onload=alert(1)>")).toBe("/clients");
  expect(clinicalReviewHref("javascript:alert(1)")).toBe("/clients");
});
