/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import {
  Button,
  Checkbox,
  Input,
  Modal,
  Radio,
  Table,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';

const BatchModelConfigModal = ({
  showBatchModelConfig,
  setShowBatchModelConfig,
  selectedChannels,
  batchModelConfigValue,
  setBatchModelConfigValue,
  batchModelConfigMode,
  setBatchModelConfigMode,
  batchModelConfigTest,
  setBatchModelConfigTest,
  batchModelConfigStream,
  setBatchModelConfigStream,
  batchModelConfigEndpointType,
  setBatchModelConfigEndpointType,
  batchModelConfigResults,
  batchModelConfigLoading,
  batchConfigChannelModels,
  t,
}) => {
  const resultColumns = [
    {
      title: t('渠道'),
      dataIndex: 'channel_name',
      render: (text, record) => `${record.channel_id} ${text || ''}`,
    },
    {
      title: t('配置验证'),
      dataIndex: 'update_success',
      render: (_, record) => {
        const ok = record.update_success && record.missing_models?.length === 0;
        return (
          <Tag color={ok ? 'green' : 'red'}>{ok ? t('成功') : t('失败')}</Tag>
        );
      },
    },
    {
      title: t('新增模型'),
      dataIndex: 'added_models',
      render: (models) => (models || []).join(', ') || '-',
    },
    {
      title: t('缺失模型'),
      dataIndex: 'missing_models',
      render: (models) => (models || []).join(', ') || '-',
    },
    {
      title: t('测试结果'),
      dataIndex: 'tests',
      render: (tests) => {
        if (!tests || tests.length === 0) {
          return '-';
        }
        return tests
          .map((item) => `${item.model}:${item.success ? 'PASS' : 'FAIL'}`)
          .join(', ');
      },
    },
  ];

  return (
    <Modal
      title={t('批量配置模型')}
      visible={showBatchModelConfig}
      onCancel={() => setShowBatchModelConfig(false)}
      maskClosable={false}
      centered={true}
      size='large'
      className='!rounded-lg'
      footer={
        <div className='flex justify-end gap-2'>
          <Button onClick={() => setShowBatchModelConfig(false)}>
            {t('关闭')}
          </Button>
          <Button
            loading={batchModelConfigLoading}
            onClick={() => batchConfigChannelModels(true)}
          >
            {t('预演')}
          </Button>
          <Button
            type='primary'
            loading={batchModelConfigLoading}
            onClick={() => batchConfigChannelModels(false)}
          >
            {t('执行配置并测试')}
          </Button>
        </div>
      }
    >
      <div className='flex flex-col gap-4'>
        <Typography.Text type='secondary'>
          {t('已选择 ${count} 个渠道').replace(
            '${count}',
            selectedChannels.length,
          )}
        </Typography.Text>

        <div>
          <Typography.Text strong>{t('新增模型列表')}</Typography.Text>
          <TextArea
            autosize={{ minRows: 3, maxRows: 6 }}
            placeholder={t(
              '可用逗号或换行分隔，例如：moonshotai/kimi-k2.6, kimi-k2.6',
            )}
            value={batchModelConfigValue}
            onChange={(value) => setBatchModelConfigValue(value)}
            className='mt-2'
          />
        </div>

        <div className='flex flex-wrap items-center gap-4'>
          <Radio.Group
            type='button'
            buttonSize='small'
            value={batchModelConfigMode}
            onChange={(e) => setBatchModelConfigMode(e.target.value)}
          >
            <Radio value='append'>{t('追加并去重')}</Radio>
            <Radio value='replace'>{t('覆盖模型列表')}</Radio>
          </Radio.Group>
          <Checkbox
            checked={batchModelConfigTest}
            onChange={(e) => setBatchModelConfigTest(e.target.checked)}
          >
            {t('配置后测试')}
          </Checkbox>
          <Checkbox
            checked={batchModelConfigStream}
            onChange={(e) => setBatchModelConfigStream(e.target.checked)}
          >
            {t('流式测试')}
          </Checkbox>
          <Input
            size='small'
            value={batchModelConfigEndpointType}
            onChange={(value) => setBatchModelConfigEndpointType(value)}
            placeholder={t('endpoint_type 可留空')}
            style={{ width: 220 }}
          />
        </div>

        {batchModelConfigResults.length > 0 ? (
          <Table
            size='small'
            pagination={false}
            columns={resultColumns}
            dataSource={batchModelConfigResults}
            rowKey='channel_id'
          />
        ) : null}
      </div>
    </Modal>
  );
};

export default BatchModelConfigModal;
