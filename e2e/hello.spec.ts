import { test, expect } from '@playwright/test';

test.describe('Hello World アプリケーションのE2Eテスト', () => {
  test.beforeEach(async ({ page }) => {
    // baseURL が playwright.config.ts で設定されているため、'/' への遷移でアプリケーションにアクセスします
    await page.goto('/');
  });

  test('ページのタイトルが正しいこと', async ({ page }) => {
    // ページのタイトルを確認
    await expect(page).toHaveTitle(/Hello World - サンプルアプリ/);
  });

  test('「Hello World」の見出しが存在すること', async ({ page }) => {
    // h1タグの中に「Hello World」というテキストが存在するか確認
    const heading = page.locator('h1');
    await expect(heading).toHaveText('Hello World');
  });

  test('カウンターボタンをクリックすると数値がインクリメントされること', async ({ page }) => {
    // 初期状態のカウンター値「0」を確認
    const counterValue = page.locator('span[x-text="count"]');
    await expect(counterValue).toHaveText('0');

    // 「カウントアップ」ボタンをクリック
    const button = page.getByRole('button', { name: 'カウントアップ' });
    await button.click();

    // クリック後のカウンター値が「1」になっているか確認
    await expect(counterValue).toHaveText('1');

    // もう一度クリックして「2」になるか確認
    await button.click();
    await expect(counterValue).toHaveText('2');
  });
});
