from argparse import Namespace
import logging
import unittest
from unittest.mock import patch, MagicMock
from vmkfstools_wrapper import extract_rdmdisk_file
from vmkfstools_wrapper import main


class TestVmkfstoolsWrapper(unittest.TestCase):

    def setUp(self):
        # Mock FileHandler to avoid creating actual log files
        self.file_handler_patcher = patch('logging.FileHandler')
        self.mock_file_handler = self.file_handler_patcher.start()
        self.mock_file_handler.return_value = MagicMock()


    def tearDown(self):
        # Stop the FileHandler patcher
        self.file_handler_patcher.stop()

    def test_extract_rdmdisk_from_rdmmetadata(self):
        rdmdisk = extract_rdmdisk_file("testdata/rdmfile")
        self.assertEqual(rdmdisk, "testdata/rdmdisktest-rdm.vmdk")

    def test_missing_extract_rdmdisk_from_rdmmetadata(self):
        rdmdisk = extract_rdmdisk_file("/dev/null")
        self.assertEqual(rdmdisk, "")

    @patch('sys.argv',
           ['vmkfstools_wrapper', '--clone', '-s', 'foo', '-t', 'bar'])
    def test_clone_args(self):
        with patch('vmkfstools_wrapper.clone') as mocked_clone:
            main()
            mocked_clone.assert_called_once_with(
                Namespace(clone=True, source_vmdk="foo", target_lun="bar",
                          task_get=False, task_clean=False, task_id=None,
                          version=False))

    @patch('sys.argv', ['vmkfstools_wrapper', '--task-get', '-i', 'foo'])
    def test_task_get(self):
        with patch('os.makedirs') as _, \
                patch('vmkfstools_wrapper.taskGet') as mocked_get:
            main()
            mocked_get.assert_called_once_with(
                Namespace(clone=False, source_vmdk=None, target_lun=None,
                          task_get=True, task_clean=False, task_id=['foo'],
                          version=False))

    @patch('sys.argv', ['vmkfstools_wrapper', '--task-clean', '-i', 'foo'])
    def test_task_clean(self):
        with patch('os.makedirs') as _, \
                patch('vmkfstools_wrapper.taskClean') as mocked_task_clean:
            main()
            mocked_task_clean.assert_called_once_with(
                Namespace(clone=False, source_vmdk=None, target_lun=None,
                          task_get=False, task_clean=True, task_id=['foo'],
                          version=False))


if __name__ == '__main__':
    unittest.main()
