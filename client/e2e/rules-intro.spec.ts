import { expect, test } from "@playwright/test";

test.describe("rules intro preview", () => {
  test("renders the standalone intro animation", async ({ page }) => {
    await page.goto("/?preview=rules&game=drawful");

    await expect(page.getByRole("heading", { name: "Jrawful Rules Preview" })).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Turn your prompt into a quick sketch" })
    ).toBeVisible();
    await expect(page.getByText(/No points yet - this is the setup\./i)).toBeVisible();
    await expect(page.getByRole("button", { name: "Back" })).toBeDisabled();

    await page.getByRole("button", { name: "Next" }).click();
    await expect(page.getByRole("heading", { name: "Write a fake prompt that sounds real" })).toBeVisible();
    await expect(page.getByText(/\+500 for each player who picks your fake prompt\./i)).toBeVisible();

    await page.getByRole("button", { name: "Next" }).click();
    await expect(page.getByRole("heading", { name: "Truth pays too" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Next" })).toBeDisabled();
  });
});
