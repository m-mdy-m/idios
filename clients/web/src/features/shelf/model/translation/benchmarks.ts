export type TranslationBenchmarkFixture = {
  id: string;
  domain: 'philosophy' | 'technical' | 'general';
  reviewStatus: 'maintainer-review-required';
  source: string;
  proposedPersianReference: string;
  terminology: Array<{ source: string; preferred: string }>;
  requiredMeaningChecks: string[];
  requiredTargetMeaningChecks: string[];
};

/**
 * Deterministic reference fixtures. Persian references are proposed baselines for
 * maintainer review, not claims of independently peer-reviewed gold translations.
 */
export const translationBenchmarkFixtures: TranslationBenchmarkFixture[] = [
  {
    id: 'rawls-preface-justice-theory',
    domain: 'philosophy',
    reviewStatus: 'maintainer-review-required',
    source: [
      'The section headings, the remarks that preface each chapter, and the index will guide the reader to the contents of the book. It seems superfluous to comment on this except to say that I have avoided extensive methodological discussions. There is a brief consideration of the nature of moral theory in §9, and of justification in §4 and §87. A short digression on the meaning of “good” is found in §62. Occasionally there are methodological comments and asides, but for the most part I try to work out a substantive theory of justice. Comparisons and contrasts with other theories, and criticisms thereof now and then, especially of utilitarianism, are viewed as means to this end.',
      'By not including most of Chapters IV–VIII in the more basic parts of the book, I do not mean to suggest that these chapters are peripheral, or merely applications. Rather, I believe that an important test of a theory of justice is how well it introduces order and system into our considered judgments over a wide range of questions. Therefore the topics of these chapters need to be taken up, and the conclusions reached modify in turn the view proposed. But in this regard the reader is more free to follow his preferences and to look at the problems which most concern him.',
    ].join('\n\n'),
    proposedPersianReference: [
      'عنوان بخش‌ها، یادداشت‌هایی که در آغاز هر فصل آمده‌اند و نمایهٔ کتاب، خواننده را در یافتن مطالب آن راهنمایی می‌کنند. جز این‌که بگویم از بحث‌های مفصل روش‌شناختی پرهیز کرده‌ام، توضیح بیشتری در این باره لازم به نظر نمی‌رسد. در بند ۹، ماهیت نظریهٔ اخلاقی را به‌اختصار بررسی کرده‌ام و در بندهای ۴ و ۸۷ به مسئلهٔ توجیه پرداخته‌ام. در بند ۶۲ نیز بحث کوتاهی دربارهٔ معنای «خوب» آمده است. گاه ملاحظات و توضیحات روش‌شناختی پراکنده‌ای مطرح می‌کنم، اما در بیشتر موارد می‌کوشم نظریهٔ محتواییِ عدالت را بپرورانم. مقایسهٔ این نظریه با نظریه‌های دیگر، بررسی تفاوت‌هایشان و نقد گهگاه آن‌ها، به‌ویژه فایده‌گرایی، را وسیله‌ای در خدمت همین هدف می‌دانم.',
      'این‌که بیشتر مطالب فصل‌های چهارم تا هشتم را در بخش‌های بنیادی‌تر کتاب نیاورده‌ام، به این معنا نیست که این فصل‌ها حاشیه‌ای یا صرفاً کاربردی‌اند. برعکس، معتقدم یکی از آزمون‌های مهم هر نظریهٔ عدالت این است که تا چه اندازه می‌تواند به داوری‌های سنجیدهٔ ما دربارهٔ طیف گسترده‌ای از مسائل، نظم و ساختار ببخشد. بنابراین باید به موضوعات این فصل‌ها نیز پرداخت؛ نتایجی هم که از این بررسی‌ها به دست می‌آیند، به نوبهٔ خود دیدگاه پیشنهادی را اصلاح می‌کنند. بااین‌حال، از این نظر، خواننده آزادی بیشتری دارد که مطابق ترجیحات خود پیش برود و به مسائلی بپردازد که برای او اهمیت بیشتری دارند.',
    ].join('\n\n'),
    terminology: [
      { source: 'substantive theory', preferred: 'نظریهٔ محتوایی' },
      { source: 'utilitarianism', preferred: 'فایده‌گرایی' },
      { source: 'considered judgments', preferred: 'داوری‌های سنجیده' },
      { source: 'peripheral', preferred: 'حاشیه‌ای' },
      { source: 'merely applications', preferred: 'صرفاً کاربردی' },
    ],
    requiredMeaningChecks: ['methodological discussions', 'not mean', 'peripheral', 'merely applications', 'modify in turn'],
    requiredTargetMeaningChecks: ['به این معنا نیست', 'حاشیه‌ای', 'صرفاً کاربردی', 'به نوبهٔ خود دیدگاه پیشنهادی را اصلاح می‌کنند'],
  },
  {
    id: 'technical-transaction-invariants',
    domain: 'technical',
    reviewStatus: 'maintainer-review-required',
    source: 'A transaction must either commit all of its changes or leave the system unchanged. If validation fails, the operation must not partially update the index, even when the cache contains an older value.',
    proposedPersianReference: 'یک تراکنش باید یا همهٔ تغییرات خود را ثبت نهایی کند یا سیستم را بدون تغییر باقی بگذارد. اگر اعتبارسنجی شکست بخورد، عملیات نباید حتی در صورتی که حافظهٔ نهان مقدار قدیمی‌تری دارد، نمایه را به‌طور ناقص به‌روزرسانی کند.',
    terminology: [
      { source: 'transaction', preferred: 'تراکنش' },
      { source: 'commit', preferred: 'ثبت نهایی' },
      { source: 'validation', preferred: 'اعتبارسنجی' },
      { source: 'index', preferred: 'نمایه' },
      { source: 'cache', preferred: 'حافظهٔ نهان' },
    ],
    requiredMeaningChecks: ['must either', 'or leave', 'If validation fails', 'must not partially update'],
    requiredTargetMeaningChecks: ['یا همهٔ تغییرات خود را ثبت نهایی کند یا سیستم را بدون تغییر باقی بگذارد', 'اگر اعتبارسنجی شکست بخورد', 'نباید', 'به‌طور ناقص به‌روزرسانی کند'],
  },
  {
    id: 'general-qualified-claim',
    domain: 'general',
    reviewStatus: 'maintainer-review-required',
    source: 'Although the first explanation is plausible, the available evidence does not establish that it is correct. A second possibility should remain open until the missing measurements are collected.',
    proposedPersianReference: 'هرچند توضیح نخست معقول به نظر می‌رسد، شواهد موجود درستی آن را ثابت نمی‌کنند. تا زمانی که اندازه‌گیری‌هایِ انجام‌نشده گردآوری نشده‌اند، باید احتمال دوم را نیز همچنان در نظر داشت.',
    terminology: [],
    requiredMeaningChecks: ['Although', 'does not establish', 'should remain open', 'until'],
    requiredTargetMeaningChecks: ['هرچند', 'درستی آن را ثابت نمی‌کنند', 'باید احتمال دوم را نیز همچنان در نظر داشت', 'تا زمانی که']
  },
];

export const translationBenchmarkRubric = [
  'Meaning and proposition fidelity: preserve who did what, conditions, comparisons and causal relations.',
  'Negation and qualification: never drop or reverse not, only, merely, although, unless, may, or similar operators.',
  'Completeness: every sentence and paragraph is represented; no source claim is silently omitted.',
  'Terminology consistency: use the same preferred equivalent when a book-specific term retains the same sense.',
  'Persian readability: natural syntax and idiom without flattening technical or philosophical distinctions.',
  'No additions: do not insert explanations, context or claims into the translated passage.',
] as const;

export type TranslationBenchmarkReport = {
  fixtureCount: number;
  domainCounts: Record<TranslationBenchmarkFixture['domain'], number>;
  deterministicChecks: 'structural-and-required-meaning-markers';
  semanticReview: {
    status: 'pending-maintainer-review';
    reviewedFixtures: 0;
    pendingFixtureIDs: string[];
    noQualityScoreClaimed: true;
  };
};

/** A reproducible report which explicitly keeps automated checks separate from human review. */
export function createTranslationBenchmarkReport(): TranslationBenchmarkReport {
  const domainCounts = { philosophy: 0, technical: 0, general: 0 };
  for (const fixture of translationBenchmarkFixtures) domainCounts[fixture.domain] += 1;
  return {
    fixtureCount: translationBenchmarkFixtures.length,
    domainCounts,
    deterministicChecks: 'structural-and-required-meaning-markers',
    semanticReview: {
      status: 'pending-maintainer-review',
      reviewedFixtures: 0,
      pendingFixtureIDs: translationBenchmarkFixtures.map((fixture) => fixture.id),
      noQualityScoreClaimed: true,
    },
  };
}

