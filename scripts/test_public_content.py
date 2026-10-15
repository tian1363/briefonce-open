import contextlib
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("public_guard", Path(__file__).with_name("check-public-content.py"))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class PublicContentTests(unittest.TestCase):
    def run_check(self, name, content):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            path = root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content)
            output, errors = io.StringIO(), io.StringIO()
            with patch.object(guard, "ROOT", root), patch.object(guard.subprocess, "check_output", return_value=(name + "\0").encode()), contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
                result = guard.check()
            return result, errors.getvalue()

    def test_public_user_instructions_are_allowed(self):
        result, _ = self.run_check("docs/user-guide.md", "# 使用手册\n选择商品图，生成后预览并下载。")
        self.assertEqual(result, 0)

    def test_restricted_documents_cannot_return_under_their_old_paths(self):
        for name in ["docs/admin-user-manual.md", "docs/assets/admin-preview.png", "docs/social/leads.md"]:
            result, _ = self.run_check(name, "placeholder")
            self.assertEqual(result, 1)

    def test_renamed_business_and_retired_feature_documents_are_detected(self):
        for text in ["毛利率与成本倍数", "CreatiBI 发布", "视频字幕换版", "local-admin@scriptagent.test"]:
            result, _ = self.run_check("docs/new-guide.md", text)
            self.assertEqual(result, 1)

    def test_secret_values_are_not_printed(self):
        value = "sk-" + "A" * 32
        result, errors = self.run_check("config.js", value)
        self.assertEqual(result, 1)
        self.assertNotIn(value, errors)

    def test_unknown_credential_shapes_in_examples_are_detected(self):
        result, _ = self.run_check("README.md", 'export MEM0_API_KEY="' + "aB7qX8hP9sL2nM3r" + '"')
        self.assertEqual(result, 1)
        result, _ = self.run_check("README.md", 'export API_KEY="your_example_model_key"')
        self.assertEqual(result, 0)


if __name__ == "__main__":
    unittest.main()
