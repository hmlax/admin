import { useRef } from 'react';
import { Alert, App, Divider } from 'antd';
import { DrawerForm, ProFormDependency, ProFormDigit, ProFormSelect, ProFormText } from '@ant-design/pro-components';
import type { ProFormInstance } from '@ant-design/pro-components';
import { useTranslation } from 'react-i18next';
import type { MuskdeerInstance, MuskdeerInstanceDraft, MuskdeerInstanceParameters } from '@/api/generated/admin/v1';
import { muskdeerReaders, saveMuskdeerInstance, useMuskdeerWrite } from '@/api/hooks/muskdeer';
import { useDrawerCloseGuard } from '@/components/common/use-drawer-close-guard';

type Values = MuskdeerInstanceDraft & { kind: string };
type Props = { current?: MuskdeerInstance; onClose: () => void; onSaved: () => void };

// Every visible parameter is sent explicitly; no template/runtime defaults merge.
export function selectedParameters(kind: string, parameters: MuskdeerInstanceParameters): MuskdeerInstanceParameters {
  const execution = parameters.execution;
  if (kind === 'market_maker') return { execution, marketMaker: parameters.marketMaker };
  if (kind === 'grid') return { execution, grid: parameters.grid };
  if (kind === 'wash_trade') return { execution, washTrade: { ...parameters.washTrade, developmentOnly: true } };
  throw new Error('Unknown strategy type');
}

export default function InstanceForm({ current, onClose, onSaved }: Props) {
  const { t } = useTranslation('muskdeer');
  const { message } = App.useApp();
  const formRef = useRef<ProFormInstance<Values>>(null);
  const save = useMuskdeerWrite(saveMuskdeerInstance);
  const { guardClose, handleValuesChange } = useDrawerCloseGuard(true, onClose);
  const required = [{ required: true, message: t('required') }];
  const decimalRules = [...required, { pattern: /^(0|[1-9]\d*)(\.\d+)?$/, message: t('decimalRequired') }];
  const decimal = (group: 'marketMaker' | 'grid' | 'washTrade', name: string, label = name) => (
    <ProFormText key={name} name={['parameters', group, name]} label={t(label)} rules={decimalRules} />
  );
  const integer = (group: 'execution' | 'marketMaker' | 'grid' | 'washTrade', name: string, min = 0) => (
    <ProFormDigit key={name} name={['parameters', group, name]} label={t(name)} rules={required} fieldProps={{ min, max: Number.MAX_SAFE_INTEGER, precision: 0 }} />
  );
  const initial: Values = current ? { ...current, kind: current.strategyType ?? '', parameters: current.parameters } : {
    kind: 'market_maker', marketDataSource: 'binance', templateRevision: 1,
    parameters: {
      execution: { pollIntervalMs: 200, operationTimeoutMs: 2000, leaseTtlMs: 10000, leverage: 10 },
      marketMaker: { depth: 20, quantityFraction: '0.1', replacementFraction: '0.1', bridgeSlots: 1 },
      grid: { maxMarketAgeMs: 3000 },
      washTrade: { developmentOnly: true, mode: 'agg_trade', intervalMs: 100, maximumEvents: 0, maximumNotional: '0', quantityFraction: '0.1' },
    },
  };
  return (
    <DrawerForm<Values> open formRef={formRef} title={t(current ? 'edit' : 'create')} initialValues={initial}
      drawerProps={{ size: 650, destroyOnHidden: true, onClose: guardClose }} onValuesChange={handleValuesChange}
      submitter={{ searchConfig: { submitText: t('save'), resetText: t('close') }, resetButtonProps: { onClick: guardClose }, submitButtonProps: { loading: save.isPending } }}
      onFinish={async (values) => {
        try {
          const parameters = selectedParameters(values.kind, values.parameters ?? {});
          await save.mutateAsync({ current, draft: { instanceKey: values.instanceKey, templateId: values.templateId, templateRevision: values.templateRevision, accountId: values.accountId, instrumentId: values.instrumentId, marketDataSource: values.marketDataSource, parameters } });
          message.success(t('saved')); onSaved(); onClose(); return true;
        } catch (error) { console.error('Save instance failed', error); message.error(t('failed')); return false; }
      }}>
      <Alert type="info" title={t('parameterNotice')} />
      <Divider>{t('identity')}</Divider>
      <ProFormText name="instanceKey" label={t('instanceKey')} disabled={!!current} rules={required} />
      <ProFormSelect name="templateId" label={t('templateId')} disabled={!!current} rules={required}
        request={async () => (await muskdeerReaders.templates({ pageSize: 200 })).items?.map((row) => ({ label: row.name, value: row.id, kind: row.strategyType, revision: row.revision })) ?? []}
        fieldProps={{ onChange: (_, option) => {
          if (option && !Array.isArray(option)) formRef.current?.setFieldsValue({ kind: option.kind, templateRevision: option.revision });
        } }} />
      <ProFormText name="kind" hidden />
      <ProFormDigit name="templateRevision" hidden />
      <ProFormSelect name="accountId" label={t('accountId')} disabled={!!current} showSearch rules={required}
        request={async ({ keyWords }) => (await muskdeerReaders.accounts({ search: keyWords, pageSize: 100 })).items?.map((row) => ({ label: `${row.symbol} · ${row.accountId}`, value: row.accountId, leverage: row.leverage })) ?? []}
        fieldProps={{ filterOption: false, onChange: (_, option) => { if (option && !Array.isArray(option)) formRef.current?.setFieldValue(['parameters', 'execution', 'leverage'], option.leverage); } }} />
      <ProFormSelect name="instrumentId" label={t('instrumentId')} disabled={!!current} showSearch rules={required}
        request={async ({ keyWords }) => (await muskdeerReaders.instruments({ search: keyWords, pageSize: 100 })).items?.map((row) => ({ label: row.symbol, value: row.id })) ?? []} fieldProps={{ filterOption: false }} />
      <ProFormSelect name="marketDataSource" label={t('marketDataSource')} disabled={!!current} rules={required} options={[{ label: 'Binance', value: 'binance' }]} />
      <Divider>{t('execution')}</Divider>
      {['pollIntervalMs', 'operationTimeoutMs', 'leaseTtlMs', 'leverage'].map((name) => integer('execution', name, 1))}
      <Divider>{t('parameters')}</Divider>
      <ProFormDependency name={['kind']}>{({ kind }) => kind === 'market_maker' ? <>
        {integer('marketMaker', 'depth', 1)}{decimal('marketMaker', 'quantityFraction')}{decimal('marketMaker', 'replacementFraction')}{integer('marketMaker', 'bridgeSlots')}
      </> : kind === 'grid' ? <>
        {['lowerPrice', 'upperPrice', 'totalBudget', 'perOrderNotional', 'inventoryLimit'].map((name) => decimal('grid', name))}
        {integer('grid', 'gridCount', 2)}{integer('grid', 'maxMarketAgeMs', 1)}
      </> : <>
        <Alert type="info" title={t('syntheticNotice')} />
        <ProFormText name={['parameters', 'washTrade', 'runId']} label={t('runId')} rules={[...required, { pattern: /^[a-z0-9-]{1,20}$/ }]} />
        <ProFormSelect name={['parameters', 'washTrade', 'mode']} label={t('mode')} rules={required} options={[{ label: t('book'), value: 'book' }, { label: t('agg_trade'), value: 'agg_trade' }]} />
        {integer('washTrade', 'intervalMs', 100)}{integer('washTrade', 'maximumEvents')}
        {decimal('washTrade', 'quantity')}{decimal('washTrade', 'maximumNotional', 'washMaximumNotional')}{decimal('washTrade', 'quantityFraction')}
      </>}</ProFormDependency>
    </DrawerForm>
  );
}
