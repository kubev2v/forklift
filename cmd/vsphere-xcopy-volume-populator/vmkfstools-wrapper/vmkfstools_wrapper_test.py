from argparse import Namespace
import unittest
from unittest.mock import patch
from vmkfstools_wrapper import extract_rdmdisk_file
from vmkfstools_wrapper import main


class TestVmkfstoolsWrapper(unittest.TestCase):

    def test_extract_rdmdisk_from_rdmmetadata(self):
        rdmdisk = extract_rdmdisk_file("testdata/rdmfile")
        self.assertEqual(rdmdisk, "testdata/rdmdisktest-rdm.vmdk")

    def test_missing_extract_rdmdisk_from_rdmmetadata(self):
        rdmdisk = extract_rdmdisk_file("/dev/null")
        self.assertEqual(rdmdisk, "")

    @patch('sys.argv',
           ['vmkfstools_wrapper', '--clone', '-s', 'foo', '-t', 'bar'])
    def test_clone_args(self):
        with patch('logging.basicConfig') as _, \
                patch('vmkfstools_wrapper.clone') as mocked_clone:
            main()
            mocked_clone.assert_called_once_with(
                Namespace(clone=True, source_vmdk="foo", target_lun="bar",
                          task_get=False, task_clean=False, task_id=None))

    @patch('sys.argv', ['vmkfstools_wrapper', '--task-get', '-i', 'foo'])
    def test_task_get(self):
        with patch('logging.basicConfig') as _, \
                patch('os.makedirs') as _, \
                patch('vmkfstools_wrapper.taskGet') as mocked_get:
            main()
            mocked_get.assert_called_once_with(
                Namespace(clone=False, source_vmdk=None, target_lun=None,
                          task_get=True, task_clean=False, task_id=['foo']))

    @patch('sys.argv', ['vmkfstools_wrapper', '--task-clean', '-i', 'foo'])
    def test_task_clean(self):
        with patch('logging.basicConfig') as _, \
                patch('os.makedirs') as _, \
                patch('vmkfstools_wrapper.taskClean') as mocked_task_clean:
            main()
            mocked_task_clean.assert_called_once_with(
                Namespace(clone=False, source_vmdk=None, target_lun=None,
                          task_get=False, task_clean=True, task_id=['foo']))


if __name__ == '__main__':
    unittest.main()
